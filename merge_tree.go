// Copyright 2026 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/aviator-co/niche-git/debug"
	"github.com/aviator-co/niche-git/internal/fetch"
	"github.com/aviator-co/niche-git/internal/merge"
	"github.com/aviator-co/niche-git/internal/resolvediff3"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/storage/memory"
)

type MergeTreeArgs struct {
	RepoURL   string `json:"repoURL"`
	Commit1   string `json:"commit1"`
	Commit2   string `json:"commit2"`
	MergeBase string `json:"mergeBase"`
}

type MergeTreeOutput struct {
	ConflictOpenFiles     []string              `json:"conflictOpenFiles"`
	ConflictResolvedFiles []string              `json:"conflictResolvedFiles"`
	BinaryConflictFiles   []string              `json:"binaryConflictFiles"`
	NonFileConflictFiles  []string              `json:"nonFileConflictFiles"`
	FetchDebugInfo        debug.FetchDebugInfo  `json:"fetchDebugInfo"`
	BlobFetchDebugInfo    *debug.FetchDebugInfo `json:"blobFetchDebugInfo"`
	Error                 string                `json:"error,omitempty"`
}

// MergeTree performs an in-memory three-way merge of Commit1 and Commit2 using
// MergeBase as the merge base and reports which files conflict. Unlike
// SquashCherryPick / LinearRebase it never creates a commit or pushes a ref, so
// it is safe to use purely to inspect whether (and where) two commits conflict.
func MergeTree(ctx context.Context, client *http.Client, args MergeTreeArgs) MergeTreeOutput {
	result, fetchDebugInfo, blobFetchDebugInfo, err := mergeTree(
		ctx,
		args.RepoURL,
		client,
		plumbing.NewHash(args.Commit1),
		plumbing.NewHash(args.Commit2),
		plumbing.NewHash(args.MergeBase),
	)
	output := MergeTreeOutput{
		FetchDebugInfo:     fetchDebugInfo,
		BlobFetchDebugInfo: blobFetchDebugInfo,
	}
	if result != nil {
		output.ConflictOpenFiles = result.ConflictOpenFiles
		output.ConflictResolvedFiles = result.ConflictResolvedFiles
		output.BinaryConflictFiles = result.BinaryConflictFiles
		output.NonFileConflictFiles = result.NonFileConflictFiles
	}
	if output.ConflictOpenFiles == nil {
		output.ConflictOpenFiles = []string{}
	}
	if output.ConflictResolvedFiles == nil {
		output.ConflictResolvedFiles = []string{}
	}
	if output.BinaryConflictFiles == nil {
		output.BinaryConflictFiles = []string{}
	}
	if output.NonFileConflictFiles == nil {
		output.NonFileConflictFiles = []string{}
	}
	if err != nil {
		output.Error = err.Error()
	}
	return output
}

type mergeTreeResult struct {
	ConflictOpenFiles     []string
	ConflictResolvedFiles []string
	BinaryConflictFiles   []string
	NonFileConflictFiles  []string
}

func mergeTree(
	ctx context.Context,
	repoURL string,
	client *http.Client,
	commit1, commit2, mergeBase plumbing.Hash,
) (*mergeTreeResult, debug.FetchDebugInfo, *debug.FetchDebugInfo, error) {
	packfilebs, fetchDebugInfo, err := fetch.FetchBlobNonePackfile(ctx, repoURL, client, []plumbing.Hash{commit1, commit2, mergeBase}, 1)
	if err != nil {
		return nil, fetchDebugInfo, nil, err
	}

	storage := memory.NewStorage()
	parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), storage)
	if err != nil {
		return nil, fetchDebugInfo, nil, fmt.Errorf("failed to parse packfile: %v", err)
	}
	if _, err := parser.Parse(); err != nil {
		return nil, fetchDebugInfo, nil, fmt.Errorf("failed to parse packfile: %v", err)
	}

	tree1, err := getTreeFromCommit(storage, commit1)
	if err != nil {
		return nil, fetchDebugInfo, nil, err
	}
	tree2, err := getTreeFromCommit(storage, commit2)
	if err != nil {
		return nil, fetchDebugInfo, nil, err
	}
	treeBase, err := getTreeFromCommit(storage, mergeBase)
	if err != nil {
		return nil, fetchDebugInfo, nil, err
	}

	// First pass collects the conflicting blob hashes so we can fetch them, then
	// the second pass runs the real diff3 resolver to classify the conflicts.
	collector := &conflictBlobCollector{}
	mergeResult, err := merge.MergeTree(storage, tree1, tree2, treeBase, collector.Resolve)
	if err != nil {
		return nil, fetchDebugInfo, nil, fmt.Errorf("failed to merge the trees: %v", err)
	}

	resolver := resolvediff3.NewDiff3Resolver(storage, "Commit1 content", "Commit2 content", ".rej", "")
	var blobFetchDebugInfo *debug.FetchDebugInfo
	if len(mergeResult.FilesConflict) != 0 {
		if len(collector.blobHashes) > 0 {
			packfilebs, fetchBlobDebugInfo, err := fetch.FetchBlobPackfile(ctx, repoURL, client, collector.blobHashes)
			blobFetchDebugInfo = &fetchBlobDebugInfo
			if err != nil {
				return nil, fetchDebugInfo, blobFetchDebugInfo, err
			}
			parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), storage)
			if err != nil {
				return nil, fetchDebugInfo, blobFetchDebugInfo, fmt.Errorf("failed to parse packfile: %v", err)
			}
			if _, err := parser.Parse(); err != nil {
				return nil, fetchDebugInfo, blobFetchDebugInfo, fmt.Errorf("failed to parse packfile: %v", err)
			}
		}
		if _, err := merge.MergeTree(storage, tree1, tree2, treeBase, resolver.Resolve); err != nil {
			return nil, fetchDebugInfo, blobFetchDebugInfo, fmt.Errorf("failed to merge the trees: %v", err)
		}
	}

	return &mergeTreeResult{
		ConflictOpenFiles:     resolver.ConflictOpenFiles,
		ConflictResolvedFiles: resolver.ConflictResolvedFiles,
		BinaryConflictFiles:   resolver.BinaryConflictFiles,
		NonFileConflictFiles:  resolver.NonFileConflictFiles,
	}, fetchDebugInfo, blobFetchDebugInfo, nil
}
