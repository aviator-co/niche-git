// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/aviator-co/niche-git/debug"
	"github.com/aviator-co/niche-git/internal/diff"
	"github.com/aviator-co/niche-git/internal/fetch"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

// FetchModifiedFiles returns the list of files that were modified between two commits.
func FetchModifiedFiles(repoURL string, client *http.Client, commitHash1, commitHash2 plumbing.Hash) ([]string, debug.FetchDebugInfo, error) {
	packfilebs, debugInfo, err := fetch.FetchBlobNonePackfile(repoURL, client, []plumbing.Hash{commitHash1, commitHash2})
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

	commit1, err := object.GetCommit(storage, commitHash1)
	if err != nil {
		return nil, debugInfo, fmt.Errorf("cannot find %q in the fetched packfile: %v", commitHash1, err)
	}
	commit2, err := object.GetCommit(storage, commitHash2)
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

	modified, err := diff.DiffTree(storage, tree1, tree2)
	if err != nil {
		return nil, debugInfo, fmt.Errorf("failed to take file diffs: %v", err)
	}
	var ret []string
	for pth := range modified {
		ret = append(ret, pth)
	}
	return ret, debugInfo, nil
}
