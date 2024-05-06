// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"

	"github.com/aviator-co/niche-git/debug"
	"github.com/aviator-co/niche-git/internal/fetch"
	"github.com/aviator-co/niche-git/internal/merge"
	"github.com/aviator-co/niche-git/internal/push"
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
}

// PushSquashCherryPick creates a new commit with the changes between the two commits and push to
// the specified ref.
func PushSquashCherryPick(
	repoURL string,
	client *http.Client,
	commitHashCherryPickFrom, commitHashCherryPickTo, commitHashCherryPickBase plumbing.Hash,
	commitMessage string,
	author, comitter object.Signature,
	ref plumbing.ReferenceName,
	currentRefhash *plumbing.Hash,
	abortOnConflict bool,
) (*PushSquashCherryPickResult, debug.FetchDebugInfo, *debug.PushDebugInfo, error) {
	packfilebs, fetchDebugInfo, err := fetch.FetchBlobNonePackfile(repoURL, client, []plumbing.Hash{commitHashCherryPickFrom, commitHashCherryPickBase, commitHashCherryPickTo})
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

	treeCPFrom, err := getTreeFromCommit(storage, commitHashCherryPickFrom)
	if err != nil {
		return nil, fetchDebugInfo, nil, err
	}
	treeCPBase, err := getTreeFromCommit(storage, commitHashCherryPickBase)
	if err != nil {
		return nil, fetchDebugInfo, nil, err
	}
	treeCPTo, err := getTreeFromCommit(storage, commitHashCherryPickTo)
	if err != nil {
		return nil, fetchDebugInfo, nil, err
	}

	mergeResult, err := merge.MergeTree(storage, treeCPFrom, treeCPTo, treeCPBase, conflictResolver)
	if err != nil {
		return nil, fetchDebugInfo, nil, fmt.Errorf("failed to merge the trees: %v", err)
	}
	cpResult := &PushSquashCherryPickResult{
		CherryPickedFiles: mergeResult.FilesPickedEntry1,
		ConflictOpenFiles: mergeResult.FilesConflict,
	}
	if abortOnConflict && len(mergeResult.FilesConflict) > 0 {
		return cpResult, fetchDebugInfo, nil, errors.New("conflict detected")
	}
	commit := &object.Commit{
		Message:      commitMessage,
		Author:       author,
		Committer:    comitter,
		TreeHash:     mergeResult.TreeHash,
		ParentHashes: []plumbing.Hash{commitHashCherryPickTo},
	}
	obj := storage.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		return cpResult, fetchDebugInfo, nil, fmt.Errorf("failed to create a commit: %v", err)
	}
	commitHash, err := storage.SetEncodedObject(obj)
	if err != nil {
		return cpResult, fetchDebugInfo, nil, fmt.Errorf("failed to create a commit: %v", err)
	}
	cpResult.CommitHash = commitHash

	var buf bytes.Buffer
	packEncoder := packfile.NewEncoder(&buf, storage, false)
	if _, err := packEncoder.Encode(append([]plumbing.Hash{commitHash}, mergeResult.NewHashes...), 0); err != nil {
		return cpResult, fetchDebugInfo, nil, fmt.Errorf("failed to create a packfile: %v", err)
	}

	pushDebugInfo, err := push.Push(repoURL, client, &buf, []push.RefUpdate{
		{
			Name:    plumbing.ReferenceName(ref),
			OldHash: currentRefhash,
			NewHash: commitHash,
		},
	})
	if err != nil {
		return cpResult, fetchDebugInfo, &pushDebugInfo, err
	}
	return cpResult, fetchDebugInfo, &pushDebugInfo, nil
}

func conflictResolver(parentPath string, cpFromEntry, cpToEntry, base *object.TreeEntry) ([]object.TreeEntry, error) {
	var ret []object.TreeEntry
	if cpFromEntry != nil {
		ret = append(ret, object.TreeEntry{Name: cpFromEntry.Name + ".from-cherry-pick", Hash: cpFromEntry.Hash, Mode: cpFromEntry.Mode})
	}
	if cpToEntry != nil {
		ret = append(ret, *cpToEntry)
	}
	if base != nil {
		ret = append(ret, object.TreeEntry{Name: base.Name + ".from-cherry-pick-base", Hash: base.Hash, Mode: base.Mode})
	}
	return ret, nil
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
