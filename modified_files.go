// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"fmt"
	"net/http"
	"path"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

type FetchModifiedFilesDebugInfo struct {
	// PackfileSize is the size of the fetched packfile in bytes.
	PackfileSize int

	// ResponseHeaders is the headers of the HTTP response when fetching the packfile.
	ResponseHeaders http.Header
}

// FetchModifiedFiles returns the list of files that were modified between two commits.
func FetchModifiedFiles(repoURL string, client *http.Client, commitHash1, commitHash2 string) ([]string, *FetchModifiedFilesDebugInfo, error) {
	packfilebs, headers, err := FetchBlobNonePackfile(repoURL, client, []string{commitHash1, commitHash2})
	debugInfo := &FetchModifiedFilesDebugInfo{
		PackfileSize:    len(packfilebs),
		ResponseHeaders: headers,
	}
	if err != nil {
		return nil, debugInfo, err
	}

	storage := memory.NewStorage()
	parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), storage)
	if err != nil {
		return nil, debugInfo, fmt.Errorf("failed to parse packfile: %v", err)
	}
	if _, err := parser.Parse(); err != nil {
		return nil, debugInfo, fmt.Errorf("failed to parse packfile: %v", err)
	}

	commit1, err := object.GetCommit(storage, plumbing.NewHash(commitHash1))
	if err != nil {
		return nil, debugInfo, fmt.Errorf("cannot find %q in the fetched packfile: %v", commitHash1, err)
	}
	commit2, err := object.GetCommit(storage, plumbing.NewHash(commitHash2))
	if err != nil {
		return nil, debugInfo, fmt.Errorf("cannot find %q in the fetched packfile: %v", commitHash2, err)
	}

	tree1, err := commit1.Tree()
	if err != nil {
		return nil, debugInfo, fmt.Errorf("cannot find the tree of %q in the fetched packfile: %v", commitHash1, err)
	}
	tree2, err := commit2.Tree()
	if err != nil {
		return nil, debugInfo, fmt.Errorf("cannot find the tree of %q in the fetched packfile: %v", commitHash2, err)
	}

	td := treeDiffer{
		storage:  storage,
		modified: map[string]bool{},
	}
	if err := td.Diff("", tree1, tree2); err != nil {
		return nil, debugInfo, fmt.Errorf("failed to take file diffs: %v", err)
	}
	var ret []string
	for pth := range td.modified {
		ret = append(ret, pth)
	}
	return ret, debugInfo, nil
}

type treeDiffer struct {
	storage  *memory.Storage
	modified map[string]bool
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
			td.handleExistOnlyInOneSide(pth, entry2)
			continue
		}
		if entry2 == nil {
			td.handleExistOnlyInOneSide(pth, entry1)
			continue
		}
		if entry1.Hash == entry2.Hash {
			// Matches the entire content. Whether it's a file or a directory, the whole
			// contents are the same.
			continue
		}
		if entry1.Mode.IsFile() && entry2.Mode.IsFile() {
			// Simply the files are different.
			td.modified[path.Join(pth, name)] = true
			continue
		}
		if !entry1.Mode.IsFile() && entry2.Mode.IsFile() {
			td.modified[path.Join(pth, name)] = true
			td.handleExistOnlyInOneSide(pth, entry1)
			continue
		}
		if entry1.Mode.IsFile() && !entry2.Mode.IsFile() {
			td.modified[path.Join(pth, name)] = true
			td.handleExistOnlyInOneSide(pth, entry2)
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

func (td *treeDiffer) handleExistOnlyInOneSide(pth string, entry *object.TreeEntry) error {
	if entry.Mode.IsFile() {
		td.modified[path.Join(pth, entry.Name)] = true
		return nil
	}
	subtree, err := object.GetTree(td.storage, entry.Hash)
	if err != nil {
		return err
	}
	subtreePath := path.Join(pth, entry.Name)
	for _, subEntry := range subtree.Entries {
		if err := td.handleExistOnlyInOneSide(subtreePath, &subEntry); err != nil {
			return err
		}
	}
	return nil
}
