// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/aviator-co/niche-git/debug"
	"github.com/aviator-co/niche-git/internal/fetch"
	"github.com/aviator-co/niche-git/internal/merge"
	"github.com/aviator-co/niche-git/internal/push"
	"github.com/aviator-co/niche-git/internal/resolvediff3"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

type PushSquashCherryPickResult struct {
	CommitHash            plumbing.Hash
	CherryPickedFiles     []string
	ConflictOpenFiles     []string
	ConflictResolvedFiles []string
	BinaryConflictFiles   []string
	NonFileConflictFiles  []string
}

// PushSquashCherryPick creates a new commit with the changes between the two commits and push to
// the specified ref.
func PushSquashCherryPick(
	ctx context.Context,
	repoURL string,
	client *http.Client,
	commitHashCherryPickFrom, commitHashCherryPickTo, commitHashCherryPickBase plumbing.Hash,
	commitMessage string,
	author, committer object.Signature,
	ref plumbing.ReferenceName,
	conflictRef *plumbing.ReferenceName,
	currentRefhash *plumbing.Hash,
	abortOnConflict bool,
) (*PushSquashCherryPickResult, debug.FetchDebugInfo, *debug.FetchDebugInfo, *debug.PushDebugInfo, error) {
	packfilebs, fetchDebugInfo, err := fetch.FetchBlobNonePackfile(ctx, repoURL, client, []plumbing.Hash{commitHashCherryPickFrom, commitHashCherryPickBase, commitHashCherryPickTo}, 1)
	if err != nil {
		return nil, fetchDebugInfo, nil, nil, err
	}

	storage := memory.NewStorage()
	parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), storage)
	if err != nil {
		return nil, fetchDebugInfo, nil, nil, fmt.Errorf("failed to parse packfile: %v", err)
	}
	if _, err := parser.Parse(); err != nil {
		return nil, fetchDebugInfo, nil, nil, fmt.Errorf("failed to parse packfile: %v", err)
	}

	treeCPFrom, err := getTreeFromCommit(storage, commitHashCherryPickFrom)
	if err != nil {
		return nil, fetchDebugInfo, nil, nil, err
	}
	treeCPBase, err := getTreeFromCommit(storage, commitHashCherryPickBase)
	if err != nil {
		return nil, fetchDebugInfo, nil, nil, err
	}
	treeCPTo, err := getTreeFromCommit(storage, commitHashCherryPickTo)
	if err != nil {
		return nil, fetchDebugInfo, nil, nil, err
	}

	collector := &conflictBlobCollector{}
	mergeResult, err := merge.MergeTree(storage, treeCPFrom, treeCPTo, treeCPBase, collector.Resolve)
	if err != nil {
		return nil, fetchDebugInfo, nil, nil, fmt.Errorf("failed to merge the trees: %v", err)
	}

	resolver := resolvediff3.NewDiff3Resolver(storage, "Cherry-pick content", "Base content", ".rej", "")
	var blobFetchDebugInfo *debug.FetchDebugInfo
	if len(mergeResult.FilesConflict) != 0 {
		// Need to fetch blobs and resolve the conflicts.
		if len(collector.blobHashes) > 0 {
			packfilebs, fetchBlobDebugInfo, err := fetch.FetchBlobPackfile(ctx, repoURL, client, collector.blobHashes)
			blobFetchDebugInfo = &fetchBlobDebugInfo
			if err != nil {
				return nil, fetchDebugInfo, blobFetchDebugInfo, nil, err
			}
			parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), storage)
			if err != nil {
				return nil, fetchDebugInfo, blobFetchDebugInfo, nil, fmt.Errorf("failed to parse packfile: %v", err)
			}
			if _, err := parser.Parse(); err != nil {
				return nil, fetchDebugInfo, blobFetchDebugInfo, nil, fmt.Errorf("failed to parse packfile: %v", err)
			}
		}
		mergeResult, err = merge.MergeTree(storage, treeCPFrom, treeCPTo, treeCPBase, resolver.Resolve)
		if err != nil {
			return nil, fetchDebugInfo, blobFetchDebugInfo, nil, fmt.Errorf("failed to merge the trees: %v", err)
		}
	}

	cpResult := &PushSquashCherryPickResult{
		CherryPickedFiles:     mergeResult.FilesPickedEntry1,
		ConflictOpenFiles:     resolver.ConflictOpenFiles,
		ConflictResolvedFiles: resolver.ConflictResolvedFiles,
		BinaryConflictFiles:   resolver.BinaryConflictFiles,
		NonFileConflictFiles:  resolver.NonFileConflictFiles,
	}
	hasConflict := len(cpResult.ConflictOpenFiles) > 0 || len(cpResult.BinaryConflictFiles) > 0 || len(cpResult.NonFileConflictFiles) > 0
	if abortOnConflict && hasConflict {
		return cpResult, fetchDebugInfo, blobFetchDebugInfo, nil, errors.New("conflict detected")
	}
	commit := &object.Commit{
		Message:      commitMessage,
		Author:       author,
		Committer:    committer,
		TreeHash:     mergeResult.TreeHash,
		ParentHashes: []plumbing.Hash{commitHashCherryPickTo},
	}
	obj := storage.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		return cpResult, fetchDebugInfo, blobFetchDebugInfo, nil, fmt.Errorf("failed to create a commit: %v", err)
	}
	commitHash, err := storage.SetEncodedObject(obj)
	if err != nil {
		return cpResult, fetchDebugInfo, blobFetchDebugInfo, nil, fmt.Errorf("failed to create a commit: %v", err)
	}
	cpResult.CommitHash = commitHash

	newHashes := []plumbing.Hash{commitHash}
	newHashes = append(newHashes, mergeResult.NewHashes...)
	newHashes = append(newHashes, resolver.NewHashes...)

	var buf bytes.Buffer
	packEncoder := packfile.NewEncoder(&buf, storage, false)
	if _, err := packEncoder.Encode(newHashes, 0); err != nil {
		return cpResult, fetchDebugInfo, blobFetchDebugInfo, nil, fmt.Errorf("failed to create a packfile: %v", err)
	}

	var destRef plumbing.ReferenceName
	if hasConflict && conflictRef != nil {
		destRef = *conflictRef
	} else {
		destRef = ref
	}

	pushDebugInfo, err := push.Push(ctx, repoURL, client, &buf, []push.RefUpdate{
		{
			Name:    destRef,
			OldHash: currentRefhash,
			NewHash: commitHash,
		},
	})
	if err != nil {
		return cpResult, fetchDebugInfo, blobFetchDebugInfo, &pushDebugInfo, err
	}
	return cpResult, fetchDebugInfo, blobFetchDebugInfo, &pushDebugInfo, nil
}

type conflictBlobCollector struct {
	blobHashes []plumbing.Hash
}

func (c *conflictBlobCollector) Resolve(parentPath string, entry1, entry2, entryBase *object.TreeEntry) ([]object.TreeEntry, error) {
	if entry1 != nil && entry1.Mode.IsFile() && entry2 != nil && entry2.Mode.IsFile() && entryBase != nil && entryBase.Mode.IsFile() {
		c.blobHashes = append(c.blobHashes, entry1.Hash, entry2.Hash, entryBase.Hash)
	}
	return nil, nil
}

func getTreeFromCommit(storage *memory.Storage, commitHash plumbing.Hash) (*object.Tree, error) {
	commit, err := object.GetCommit(storage, commitHash)
	if err != nil {
		return nil, fmt.Errorf("cannot find %q in the fetched packfile: %v", commitHash.String(), err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("cannot find the tree of %q in the fetched packfile: %v", commitHash.String(), err)
	}
	return tree, nil
}
