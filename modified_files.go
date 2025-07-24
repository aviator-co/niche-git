// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/aviator-co/niche-git/debug"
	"github.com/aviator-co/niche-git/internal/diff"
	"github.com/aviator-co/niche-git/internal/fetch"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

type GetModifiedFilesArgs struct {
	RepoURL     string `json:"repoURL"`
	CommitHash1 string `json:"commitHash1"`
	CommitHash2 string `json:"commitHash2"`
}

type GetModifiedFilesOutput struct {
	Files     []string             `json:"files"`
	DebugInfo debug.FetchDebugInfo `json:"debugInfo"`
	Error     string               `json:"error,omitempty"`
}

func GetModifiedFiles(ctx context.Context, client *http.Client, args GetModifiedFilesArgs) GetModifiedFilesOutput {
	files, debugInfo, fetchErr := fetchModifiedFiles(
		ctx,
		args.RepoURL,
		client,
		plumbing.NewHash(args.CommitHash1),
		plumbing.NewHash(args.CommitHash2),
	)
	if files == nil {
		// Always create an empty slice for JSON output.
		files = []string{}
	}
	output := GetModifiedFilesOutput{
		Files:     files,
		DebugInfo: debugInfo,
	}
	sort.Strings(output.Files)
	if fetchErr != nil {
		output.Error = fetchErr.Error()
	}
	return output
}

func fetchModifiedFiles(ctx context.Context, repoURL string, client *http.Client, commitHash1, commitHash2 plumbing.Hash) ([]string, debug.FetchDebugInfo, error) {
	packfilebs, debugInfo, err := fetch.FetchBlobNonePackfile(ctx, repoURL, client, []plumbing.Hash{commitHash1, commitHash2}, 1)
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
