// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package diff

import (
	"path"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// BlobHashes represents the hashes of two blobs.
type BlobHashes struct {
	BlobHash1 plumbing.Hash
	BlobHash2 plumbing.Hash
}

// DiffTree returns the diff of two trees.
func DiffTree(storage storer.EncodedObjectStorer, tree1, tree2 *object.Tree) (map[string]BlobHashes, error) {
	td := &treeDiffer{
		storage:  storage,
		modified: map[string]BlobHashes{},
	}
	if err := td.Diff("", tree1, tree2); err != nil {
		return nil, err
	}
	return td.modified, nil
}

type treeDiffer struct {
	storage  storer.EncodedObjectStorer
	modified map[string]BlobHashes
}

func (td *treeDiffer) Diff(pth string, tree1, tree2 *object.Tree) error {
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

	for name := range names {
		entry1 := entries1[name]
		entry2 := entries2[name]
		if entry1 == nil {
			td.handleExistOnlyInOneSide(pth, entry2, false)
			continue
		}
		if entry2 == nil {
			td.handleExistOnlyInOneSide(pth, entry1, true)
			continue
		}
		if entry1.Hash == entry2.Hash {
			// Matches the entire content. Whether it's a file or a directory, the whole
			// contents are the same.
			continue
		}
		if entry1.Mode.IsFile() && entry2.Mode.IsFile() {
			// Simply the files are different.
			td.modified[path.Join(pth, name)] = BlobHashes{entry1.Hash, entry2.Hash}
			continue
		}
		if !entry1.Mode.IsFile() && entry2.Mode.IsFile() {
			td.modified[path.Join(pth, name)] = BlobHashes{plumbing.ZeroHash, entry2.Hash}
			td.handleExistOnlyInOneSide(pth, entry1, true)
			continue
		}
		if entry1.Mode.IsFile() && !entry2.Mode.IsFile() {
			td.modified[path.Join(pth, name)] = BlobHashes{entry1.Hash, plumbing.ZeroHash}
			td.handleExistOnlyInOneSide(pth, entry2, false)
			continue
		}
		// Both are directories.
		subtree1, err := object.GetTree(td.storage, entry1.Hash)
		if err != nil {
			return err
		}
		subtree2, err := object.GetTree(td.storage, entry2.Hash)
		if err != nil {
			return err
		}
		if err := td.Diff(path.Join(pth, name), subtree1, subtree2); err != nil {
			return err
		}
	}
	return nil
}

func (td *treeDiffer) handleExistOnlyInOneSide(pth string, entry *object.TreeEntry, isTree1 bool) error {
	if entry.Mode.IsFile() {
		if isTree1 {
			td.modified[path.Join(pth, entry.Name)] = BlobHashes{entry.Hash, plumbing.ZeroHash}
		} else {
			td.modified[path.Join(pth, entry.Name)] = BlobHashes{plumbing.ZeroHash, entry.Hash}
		}
		return nil
	}
	subtree, err := object.GetTree(td.storage, entry.Hash)
	if err != nil {
		return err
	}
	subtreePath := path.Join(pth, entry.Name)
	for _, subEntry := range subtree.Entries {
		if err := td.handleExistOnlyInOneSide(subtreePath, &subEntry, isTree1); err != nil {
			return err
		}
	}
	return nil
}
