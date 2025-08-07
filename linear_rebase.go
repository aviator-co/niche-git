// Copyright 2025 Aviator Technologies, Inc.
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
	"github.com/aviator-co/niche-git/internal/push"
	"github.com/aviator-co/niche-git/internal/resolvediff3"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

type LinearRebaseArgs struct {
	RepoURL string `json:"repoURL"`

	BaseCommit        string `json:"mergeBase"`
	DestinationCommit string `json:"destinationCommit"`

	// Refs is a list of references to rebase. The first element is the first ref to rebase.
	Refs []string `json:"refs"`
}

type LinearRebaseOutput struct {
	LinearRebaseResults []*LinearRebaseResult   `json:"linearRebaseResults"`
	LsRefsDebugInfo     *debug.LsRefsDebugInfo  `json:"lsRefsDebugInfo"`
	FetchDebugInfos     []*debug.FetchDebugInfo `json:"fetchDebugInfos"`
	PushDebugInfos      *debug.PushDebugInfo    `json:"pushDebugInfo"`
	Error               string                  `json:"error,omitempty"`
}

type LinearRebaseResult struct {
	Ref                   string   `json:"ref"`
	CommitHash            string   `json:"commitHash"`
	ConflictOpenFiles     []string `json:"conflictOpenFiles"`
	ConflictResolvedFiles []string `json:"conflictResolvedFiles"`
	BinaryConflictFiles   []string `json:"binaryConflictFiles"`
	NonFileConflictFiles  []string `json:"nonFileConflictFiles"`
}

func LinearRebase(ctx context.Context, client *http.Client, args LinearRebaseArgs) LinearRebaseOutput {
	refs, lsRefsDebugInfo, err := lsRefs(ctx, args.RepoURL, client, args.Refs)
	if err != nil {
		return LinearRebaseOutput{LsRefsDebugInfo: &lsRefsDebugInfo, Error: err.Error()}
	}

	lr := &linearRebase{
		client:            client,
		repoURL:           args.RepoURL,
		baseCommit:        plumbing.NewHash(args.BaseCommit),
		destinationCommit: args.DestinationCommit,
		refs:              make(map[string]plumbing.Hash),
		storage:           memory.NewStorage(),
	}
	for _, ref := range refs {
		lr.refs[ref.Name] = plumbing.NewHash(ref.Hash)
	}
	if len(lr.refs) != len(args.Refs) {
		return LinearRebaseOutput{
			LsRefsDebugInfo: &lsRefsDebugInfo,
			Error:           "The number of refs returned does not match the number of refs requested",
		}
	}
	var wantHashes []plumbing.Hash
	for _, hash := range lr.refs {
		wantHashes = append(wantHashes, hash)
	}
	packfilebs, fetchDebugInfo, err := fetch.FetchBlobNonePackfile(ctx, args.RepoURL, client, wantHashes, 0)
	if err != nil {
		return LinearRebaseOutput{
			LsRefsDebugInfo: &lsRefsDebugInfo,
			FetchDebugInfos: []*debug.FetchDebugInfo{&fetchDebugInfo},
			Error:           "The number of refs returned does not match the number of refs requested",
		}
	}
	parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), lr.storage)
	if err != nil {
		return LinearRebaseOutput{
			LsRefsDebugInfo: &lsRefsDebugInfo,
			FetchDebugInfos: []*debug.FetchDebugInfo{&fetchDebugInfo},
			Error:           fmt.Sprintf("failed to create packfile parser: %v", err),
		}
	}
	if _, err := parser.Parse(); err != nil {
		return LinearRebaseOutput{
			LsRefsDebugInfo: &lsRefsDebugInfo,
			FetchDebugInfos: []*debug.FetchDebugInfo{&fetchDebugInfo},
			Error:           fmt.Sprintf("failed to parse packfile: %v", err),
		}
	}
	err = lr.run(ctx)
	output := LinearRebaseOutput{
		LinearRebaseResults: lr.linearRebaseResults,
		LsRefsDebugInfo:     &lsRefsDebugInfo,
		FetchDebugInfos:     []*debug.FetchDebugInfo{&fetchDebugInfo},
		PushDebugInfos:      lr.pushDebugInfo,
	}
	if err != nil {
		output.Error = err.Error()
	}
	return output
}

type branchCommits struct {
	ref string
	// commits is a list of commits in the branch, ordered from newest to oldest.
	commits []plumbing.Hash

	// Since linear rebase expects a linear history, there's at most one parent and at most one
	// child branch.
	parentBranch *branchCommits
	childBranch  *branchCommits
}

type linearRebase struct {
	client              *http.Client
	repoURL             string
	baseCommit          plumbing.Hash
	destinationCommit   string
	refs                map[string]plumbing.Hash
	storage             *memory.Storage
	fetchDebugInfos     []*debug.FetchDebugInfo
	pushDebugInfo       *debug.PushDebugInfo
	linearRebaseResults []*LinearRebaseResult
	newObjectHashes     []plumbing.Hash
}

func (lr *linearRebase) run(ctx context.Context) error {
	branches, err := lr.findBranches()
	if err != nil {
		return fmt.Errorf("failed to find branches: %w", err)
	}
	nextBaseCommit := lr.baseCommit
	for _, branch := range branches {
		nextBaseCommit, err = lr.rebaseBranch(ctx, nextBaseCommit, branch)
		if err != nil {
			return fmt.Errorf("failed to rebase branch %s: %w", branch.ref, err)
		}
	}
	if err := lr.push(ctx); err != nil {
		return fmt.Errorf("failed to push changes: %w", err)
	}
	return nil
}

// findBranches finds all branches in the repository based on the refs provided. The returned
// branches are ordered from the root branch to the leaf branch.
func (lr *linearRebase) findBranches() ([]*branchCommits, error) {
	hashToBranch := make(map[plumbing.Hash]*branchCommits)
	for ref, hash := range lr.refs {
		branch := &branchCommits{
			ref:     ref,
			commits: []plumbing.Hash{hash},
		}
		hashToBranch[hash] = branch
	}
	for hash, branch := range hashToBranch {
		currentHash := hash
		for {
			commit, err := object.GetCommit(lr.storage, currentHash)
			if err != nil {
				return nil, fmt.Errorf("failed to get commit %s: %w", currentHash.String(), err)
			}
			if len(commit.ParentHashes) != 1 {
				return nil, fmt.Errorf("branch %s has multiple parents: %s", branch.ref, commit.ParentHashes)
			}
			parentHash := commit.ParentHashes[0]
			if parentBranch, ok := hashToBranch[parentHash]; ok {
				if parentBranch.childBranch != nil {
					return nil, fmt.Errorf("branch %s has multiple children: %s", parentBranch.ref, branch.ref)
				}
				branch.parentBranch = parentBranch
				parentBranch.childBranch = branch
				break
			}
			if parentHash == lr.baseCommit {
				// This is the base commit, so we can stop here.
				break
			}
			branch.commits = append(branch.commits, parentHash)
			currentHash = parentHash
		}
	}
	var branches []*branchCommits
	for _, branch := range hashToBranch {
		if branch.parentBranch == nil {
			// This is a root branch.
			branches = append(branches, branch)
		}
	}
	if len(branches) == 0 {
		return nil, fmt.Errorf("no root branches found")
	}
	if len(branches) > 1 {
		return nil, fmt.Errorf("multiple root branches found: %v", branches)
	}
	currentBranch := branches[0]
	for currentBranch.childBranch != nil {
		branches = append(branches, currentBranch.childBranch)
		currentBranch = currentBranch.childBranch
	}
	return branches, nil
}

func (lr *linearRebase) rebaseBranch(ctx context.Context, destCommit plumbing.Hash, branch *branchCommits) (plumbing.Hash, error) {
	result := &LinearRebaseResult{
		Ref: branch.ref,
	}
	lr.linearRebaseResults = append(lr.linearRebaseResults, result)

	for i := range branch.commits {
		// Reverse the order of commits to process them from oldest to newest.
		commitHash := branch.commits[len(branch.commits)-1-i]

		newCommitHash, err := lr.putCommit(ctx, destCommit, commitHash, result)
		if err != nil {
			return plumbing.ZeroHash, fmt.Errorf("failed to put commit %s: %v", commitHash.String(), err)
		}
		result.CommitHash = newCommitHash.String()
		destCommit = newCommitHash
	}
	return destCommit, nil
}

func (lr *linearRebase) putCommit(ctx context.Context, destCommitHash, targetCommitHash plumbing.Hash, result *LinearRebaseResult) (plumbing.Hash, error) {
	targetCommit, err := object.GetCommit(lr.storage, targetCommitHash)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to get target commit %s: %v", targetCommitHash, err)
	}

	treeTarget, err := lr.getTreeFromCommit(targetCommitHash)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to get tree from commit %s: %v", targetCommitHash, err)
	}
	treeTargetParent, err := lr.getTreeFromCommit(targetCommit.ParentHashes[0])
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to get tree from parent commit %s: %v", targetCommit.ParentHashes[0], err)
	}
	treeDest, err := lr.getTreeFromCommit(destCommitHash)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to get tree from commit %s: %v", destCommitHash, err)
	}

	collector := &conflictBlobCollector{}
	mergeResult, err := merge.MergeTree(lr.storage, treeTarget, treeDest, treeTargetParent, collector.Resolve)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to merge the trees: %v", err)
	}

	resolver := resolvediff3.NewDiff3Resolver(lr.storage, "Rebase content", "Base content", ".rej", "")
	if len(mergeResult.FilesConflict) != 0 {
		// Need to fetch blobs and resolve the conflicts.
		if len(collector.blobHashes) > 0 {
			packfilebs, fetchBlobDebugInfo, err := fetch.FetchBlobPackfile(ctx, lr.repoURL, lr.client, collector.blobHashes)
			lr.fetchDebugInfos = append(lr.fetchDebugInfos, &fetchBlobDebugInfo)
			if err != nil {
				return plumbing.ZeroHash, fmt.Errorf("failed to fetch blobs for conflict resolution: %v", err)
			}
			parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), lr.storage)
			if err != nil {
				return plumbing.ZeroHash, fmt.Errorf("failed to create packfile parser: %v", err)
			}
			if _, err := parser.Parse(); err != nil {
				return plumbing.ZeroHash, fmt.Errorf("failed to parse packfile: %v", err)
			}
		}
		mergeResult, err = merge.MergeTree(lr.storage, treeTarget, treeDest, treeTargetParent, collector.Resolve)
		if err != nil {
			return plumbing.ZeroHash, fmt.Errorf("failed to merge the trees after fetching blobs: %v", err)
		}
	}

	result.ConflictOpenFiles = append(result.ConflictOpenFiles, resolver.ConflictOpenFiles...)
	result.ConflictResolvedFiles = append(result.ConflictResolvedFiles, resolver.ConflictResolvedFiles...)
	result.BinaryConflictFiles = append(result.BinaryConflictFiles, resolver.BinaryConflictFiles...)
	result.NonFileConflictFiles = append(result.NonFileConflictFiles, resolver.NonFileConflictFiles...)
	if len(result.ConflictOpenFiles) > 0 || len(result.BinaryConflictFiles) > 0 || len(result.NonFileConflictFiles) > 0 {
		return plumbing.ZeroHash, fmt.Errorf(
			"conflicts found in branch %s %s: open files: %v, binary files: %v, non-file conflicts: %v",
			result.Ref, targetCommitHash.String(), result.ConflictOpenFiles, result.BinaryConflictFiles, result.NonFileConflictFiles,
		)
	}
	newCommit := &object.Commit{
		Message:      targetCommit.Message,
		Author:       targetCommit.Author,
		Committer:    targetCommit.Committer,
		TreeHash:     mergeResult.TreeHash,
		ParentHashes: []plumbing.Hash{destCommitHash},
	}
	obj := lr.storage.NewEncodedObject()
	if err := newCommit.Encode(obj); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to encode commit: %v", err)
	}
	newCommitHash, err := lr.storage.SetEncodedObject(obj)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to store commit: %v", err)
	}
	lr.newObjectHashes = append(lr.newObjectHashes, newCommitHash)
	lr.newObjectHashes = append(lr.newObjectHashes, mergeResult.NewHashes...)
	lr.newObjectHashes = append(lr.newObjectHashes, resolver.NewHashes...)
	return newCommitHash, nil
}

func (lr *linearRebase) push(ctx context.Context) error {
	var buf bytes.Buffer
	packEncoder := packfile.NewEncoder(&buf, lr.storage, false)
	if _, err := packEncoder.Encode(lr.newObjectHashes, 0); err != nil {
		return fmt.Errorf("failed to create a packfile: %v", err)
	}
	var refUpdates []push.RefUpdate
	for _, result := range lr.linearRebaseResults {
		oldHash := lr.refs[result.Ref]
		refUpdates = append(refUpdates, push.RefUpdate{
			Name:    plumbing.ReferenceName(result.Ref),
			OldHash: &oldHash,
			NewHash: plumbing.NewHash(result.CommitHash),
		})
	}

	pushDebugInfo, err := push.Push(ctx, lr.repoURL, lr.client, &buf, refUpdates)
	if err != nil {
		return fmt.Errorf("failed to push changes: %v", err)
	}
	lr.pushDebugInfo = &pushDebugInfo
	return nil
}

func (lr *linearRebase) getTreeFromCommit(commitHash plumbing.Hash) (*object.Tree, error) {
	commit, err := object.GetCommit(lr.storage, commitHash)
	if err != nil {
		return nil, fmt.Errorf("cannot find %q in the fetched packfile: %v", commitHash.String(), err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("cannot find the tree of %q in the fetched packfile: %v", commitHash.String(), err)
	}
	return tree, nil
}
