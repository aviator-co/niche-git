// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package merge

import (
	"fmt"
	"path"
	"sort"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// MergeResult represents the result of a merge operation.
type MergeResult struct {
	// NewHashes are the OIDs that are newly created as a part of the merge operation.
	//
	// These include only the tree hashes that are newly created in the operation. If the
	// resolver creates new blobs or trees, they are not included in this list.
	NewHashes []plumbing.Hash

	FilesPickedEntry1  []string
	FilesPickedEntry2  []string
	FilesPickedEntry12 []string
	FilesConflict      []string

	// Tree is the result of the merge.
	TreeHash plumbing.Hash
}

type resolver = func(parentPath string, entry1, entry2, entryBase *object.TreeEntry) ([]object.TreeEntry, error)

// MergeTree executes a three-way merge of two trees.
func MergeTree(
	storage storer.EncodedObjectStorer,
	tree1, tree2 *object.Tree,
	mergeBase *object.Tree,
	conflictResolver resolver,
) (*MergeResult, error) {
	tm := &treeMerger{
		storage:          storage,
		conflictResolver: conflictResolver,
	}
	treeHash, err := tm.Merge(tree1, tree2, mergeBase)
	if err != nil {
		return nil, err
	}
	return &MergeResult{
		NewHashes:          tm.newHashes,
		FilesPickedEntry1:  tm.filesPickedEntry1,
		FilesPickedEntry2:  tm.filesPickedEntry2,
		FilesPickedEntry12: tm.filesPickedEntry12,
		FilesConflict:      tm.filesConflict,
		TreeHash:           treeHash,
	}, nil
}

type treeMerger struct {
	storage          storer.EncodedObjectStorer
	conflictResolver resolver

	newHashes []plumbing.Hash

	filesPickedEntry1  []string
	filesPickedEntry2  []string
	filesPickedEntry12 []string
	filesConflict      []string
}

func (tm *treeMerger) Merge(tree1, tree2, mergeBase *object.Tree) (plumbing.Hash, error) {
	// Short-circuit if the trees are the same.
	if mergeBase != nil {
		if tree1.Hash != mergeBase.Hash && tree2.Hash == mergeBase.Hash {
			return tree1.Hash, nil
		}
		if tree1.Hash == mergeBase.Hash && tree2.Hash != mergeBase.Hash {
			return tree2.Hash, nil
		}
		if tree1.Hash == mergeBase.Hash && tree2.Hash == mergeBase.Hash {
			return mergeBase.Hash, nil
		}
	}
	if tree1.Hash == tree2.Hash {
		// Doesn't matter which tree we return, they are the same.
		return tree1.Hash, nil
	}
	return tm.mergeInternal("", tree1, tree2, mergeBase)
}

func (tm *treeMerger) mergeInternal(pth string, tree1, tree2, mergeBase *object.Tree) (plumbing.Hash, error) {
	names := map[string]bool{}
	entries1 := map[string]*object.TreeEntry{}
	for _, entry := range tree1.Entries {
		entries1[entry.Name] = &entry
		names[entry.Name] = true
	}
	entries2 := map[string]*object.TreeEntry{}
	for _, entry := range tree2.Entries {
		entries2[entry.Name] = &entry
		names[entry.Name] = true
	}
	baseEntries := map[string]*object.TreeEntry{}
	if mergeBase != nil {
		for _, entry := range mergeBase.Entries {
			baseEntries[entry.Name] = &entry
			names[entry.Name] = true
		}
	}

	var resultEntries []object.TreeEntry
	for name := range names {
		entry1 := entries1[name]
		entry2 := entries2[name]
		entryBase := baseEntries[name]
		switch checkConflictType(entry1, entry2, entryBase) {
		case conflictTypeNoChange:
			resultEntries = append(resultEntries, *entryBase)
		case conflictTypeTakeChange1:
			tm.filesPickedEntry1 = append(tm.filesPickedEntry1, path.Join(pth, name))
			if entry1 != nil {
				resultEntries = append(resultEntries, *entry1)
			}
		case conflictTypeTakeChange2:
			tm.filesPickedEntry2 = append(tm.filesPickedEntry2, path.Join(pth, name))
			if entry2 != nil {
				resultEntries = append(resultEntries, *entry2)
			}
		case conflictTypeSameChange:
			tm.filesPickedEntry12 = append(tm.filesPickedEntry12, path.Join(pth, name))
			if entry1 != nil {
				resultEntries = append(resultEntries, *entry1)
			}
		case conflictTypeConflict:
			// Both have changed, so we need to resolve the conflict. If both of them
			// are trees, we need to recurse. Otherwise, we can pass it to the conflict
			// resolver.
			if (entry1 != nil && entry1.Mode == filemode.Dir) && (entry2 != nil && entry2.Mode == filemode.Dir) {
				// Recurse.
				entry1Tree, err := object.GetTree(tm.storage, entry1.Hash)
				if err != nil {
					return plumbing.ZeroHash, fmt.Errorf("cannot get a subtree: %v", err)
				}
				entry2Tree, err := object.GetTree(tm.storage, entry2.Hash)
				if err != nil {
					return plumbing.ZeroHash, fmt.Errorf("cannot get a subtree: %v", err)
				}
				var entryBaseTree *object.Tree
				if entryBase != nil && entryBase.Mode == filemode.Dir {
					entryBaseTree, err = object.GetTree(tm.storage, entryBase.Hash)
					if err != nil {
						return plumbing.ZeroHash, fmt.Errorf("cannot get a subtree: %v", err)
					}
				}
				treeHash, err := tm.mergeInternal(path.Join(pth, name), entry1Tree, entry2Tree, entryBaseTree)
				if err != nil {
					return plumbing.ZeroHash, err
				}
				resultEntries = append(resultEntries, object.TreeEntry{Name: name, Mode: filemode.Dir, Hash: treeHash})
			} else {
				tm.filesConflict = append(tm.filesConflict, path.Join(pth, name))
				resolvedEntries, err := tm.conflictResolver(pth, entry1, entry2, entryBase)
				if err != nil {
					return plumbing.ZeroHash, fmt.Errorf("Cannot resolve conflict: %v", err)
				}
				resultEntries = append(resultEntries, resolvedEntries...)
			}
		}
	}
	sort.Sort(object.TreeEntrySorter(resultEntries))
	newTree := object.Tree{Entries: resultEntries}
	o := tm.storage.NewEncodedObject()
	if err := newTree.Encode(o); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("Cannot create a new tree entry: %v", err)
	}
	newTreeHash, err := tm.storage.SetEncodedObject(o)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("Cannot save the new tree entry: %v", err)
	}
	tm.newHashes = append(tm.newHashes, newTreeHash)
	return newTreeHash, nil
}

type conflictType int

const (
	// See https://softwareswirl.blogspot.com/2022/09/beyond-three-way-merge.html
	conflictTypeNoChange conflictType = iota
	conflictTypeTakeChange1
	conflictTypeTakeChange2
	conflictTypeSameChange
	conflictTypeConflict
)

func checkConflictType(entry1, entry2, entryBase *object.TreeEntry) conflictType {
	entry1Changed := hasChange(entry1, entryBase)
	entry2Changed := hasChange(entry2, entryBase)
	if !entry1Changed && !entry2Changed {
		return conflictTypeNoChange
	}
	if entry1Changed && !entry2Changed {
		return conflictTypeTakeChange1
	}
	if !entry1Changed && entry2Changed {
		return conflictTypeTakeChange2
	}
	if !hasChange(entry1, entry2) {
		return conflictTypeSameChange
	}
	return conflictTypeConflict
}

func hasChange(entry1, entry2 *object.TreeEntry) bool {
	if entry1 == nil && entry2 != nil {
		return true
	}
	if entry1 != nil && entry2 == nil {
		return true
	}
	if entry1 == nil && entry2 == nil {
		return false
	}
	if entry1.Mode == entry2.Mode && entry1.Hash == entry2.Hash {
		return false
	}
	return true
}
