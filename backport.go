// Copyright 2025 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"

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

type BackportArgs struct {
	RepoURL         string   `json:"repoURL"`
	BaseCommitHash  string   `json:"baseCommitHash"`
	BackportCommits []string `json:"backportCommits"`
	Ref             string   `json:"ref"`
	CurrentRefHash  string   `json:"currentRefHash"`
}

type BackportCommandResult struct {
	CommitHash              string   `json:"commitHash"`
	ConflictResolvedFiles   []string `json:"conflictResolvedFiles"`
	ConflictUnresolvedFiles []string `json:"conflictUnresolvedFiles"`
}

type BackportOutput struct {
	CommandResults  []BackportCommandResult `json:"commandResults"`
	FetchDebugInfos []*debug.FetchDebugInfo `json:"fetchDebugInfos"`
	PushDebugInfo   *debug.PushDebugInfo    `json:"pushDebugInfo"`
	Error           string                  `json:"error,omitempty"`
}

func Backport(ctx context.Context, client *http.Client, args BackportArgs) BackportOutput {
	s := &backport{
		client:  client,
		args:    args,
		storage: memory.NewStorage(),
	}
	err := s.run(ctx)
	output := BackportOutput{
		CommandResults:  s.commandResults,
		FetchDebugInfos: s.fetchDebugInfos,
		PushDebugInfo:   s.pushDebugInfo,
	}
	if err != nil {
		output.Error = err.Error()
	}
	return output
}

type backport struct {
	client          *http.Client
	args            BackportArgs
	storage         *memory.Storage
	fetchDebugInfos []*debug.FetchDebugInfo
	pushDebugInfo   *debug.PushDebugInfo
	newObjects      []plumbing.Hash
	commandResults  []BackportCommandResult
}

func (s *backport) run(ctx context.Context) error {
	if err := s.fetchCommits(ctx); err != nil {
		return err
	}
	currentCommitHash := plumbing.NewHash(s.args.BaseCommitHash)
	for _, commitStr := range s.args.BackportCommits {
		commitHash := plumbing.NewHash(commitStr)
		commandResult, err := s.cherrypickCommit(ctx, currentCommitHash, commitHash)
		s.commandResults = append(s.commandResults, commandResult)
		if err != nil {
			return err
		}
		currentCommitHash = plumbing.NewHash(commandResult.CommitHash)
	}
	if err := s.push(ctx, currentCommitHash); err != nil {
		return err
	}
	return nil
}

func (s *backport) fetchCommits(ctx context.Context) error {
	hashes := make(map[plumbing.Hash]bool)
	hashes[plumbing.NewHash(s.args.BaseCommitHash)] = true
	for _, commitStr := range s.args.BackportCommits {
		hashes[plumbing.NewHash(commitStr)] = true
	}
	packfilebs, fetchDebugInfo, err := fetch.FetchBlobNonePackfile(ctx, s.args.RepoURL, s.client, slices.AppendSeq(make([]plumbing.Hash, 0, len(hashes)), maps.Keys(hashes)), 2)
	s.fetchDebugInfos = append(s.fetchDebugInfos, &fetchDebugInfo)
	if err != nil {
		return err
	}

	parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), s.storage)
	if err != nil {
		return fmt.Errorf("failed to parse packfile: %v", err)
	}
	if _, err := parser.Parse(); err != nil {
		return fmt.Errorf("failed to parse packfile: %v", err)
	}
	return nil
}

func (s *backport) cherrypickCommit(ctx context.Context, currentCommitHash plumbing.Hash, commitHash plumbing.Hash) (BackportCommandResult, error) {
	currentCommit, err := object.GetCommit(s.storage, currentCommitHash)
	if err != nil {
		return BackportCommandResult{}, fmt.Errorf("cannot find %q in the fetched packfile: %v", currentCommitHash.String(), err)
	}
	treeDestination, err := currentCommit.Tree()
	if err != nil {
		return BackportCommandResult{}, fmt.Errorf("cannot find the tree of %q in the fetched packfile: %v", currentCommitHash.String(), err)
	}
	targetCommit, err := object.GetCommit(s.storage, commitHash)
	if err != nil {
		return BackportCommandResult{}, fmt.Errorf("cannot find %q in the fetched packfile: %v", commitHash.String(), err)
	}
	if targetCommit.NumParents() != 1 {
		return BackportCommandResult{}, fmt.Errorf("commit %q has %d parents, expected 1", commitHash.String(), targetCommit.NumParents())
	}
	parentCommit, err := targetCommit.Parent(0)
	if err != nil {
		return BackportCommandResult{}, fmt.Errorf("cannot find the parent of %q in the fetched packfile: %v", commitHash.String(), err)
	}
	treeNewContent, err := targetCommit.Tree()
	if err != nil {
		return BackportCommandResult{}, fmt.Errorf("cannot find the tree of %q in the fetched packfile: %v", commitHash.String(), err)
	}
	treeMergeBase, err := parentCommit.Tree()
	if err != nil {
		return BackportCommandResult{}, fmt.Errorf("cannot find the tree of %q in the fetched packfile: %v", parentCommit.Hash.String(), err)
	}

	collector := &conflictBlobCollector{}
	mergeResult, err := merge.MergeTree(s.storage, treeNewContent, treeDestination, treeMergeBase, collector.Resolve)
	if err != nil {
		return BackportCommandResult{}, fmt.Errorf("failed to merge the trees: %v", err)
	}
	resolver := resolvediff3.NewDiff3Resolver(s.storage, "Squash content", "Base content", ".rej", "")
	if len(mergeResult.FilesConflict) != 0 {
		// Need to fetch blobs and resolve the conflicts.
		if len(collector.blobHashes) > 0 {
			var missingBlobHashes []plumbing.Hash
			for _, hash := range collector.blobHashes {
				if _, ok := s.storage.Blobs[hash]; !ok {
					missingBlobHashes = append(missingBlobHashes, hash)
				}
			}
			packfilebs, fetchBlobDebugInfo, err := fetch.FetchBlobPackfile(ctx, s.args.RepoURL, s.client, missingBlobHashes)
			s.fetchDebugInfos = append(s.fetchDebugInfos, &fetchBlobDebugInfo)
			if err != nil {
				return BackportCommandResult{}, err
			}
			parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), s.storage)
			if err != nil {
				return BackportCommandResult{}, fmt.Errorf("failed to parse packfile: %v", err)
			}
			if _, err := parser.Parse(); err != nil {
				return BackportCommandResult{}, fmt.Errorf("failed to parse packfile: %v", err)
			}
		}
		mergeResult, err = merge.MergeTree(s.storage, treeNewContent, treeDestination, treeMergeBase, resolver.Resolve)
		if err != nil {
			return BackportCommandResult{}, fmt.Errorf("failed to merge the trees: %v", err)
		}
	}
	var conflictedFiles []string
	conflictedFiles = append(conflictedFiles, resolver.ConflictOpenFiles...)
	conflictedFiles = append(conflictedFiles, resolver.BinaryConflictFiles...)
	conflictedFiles = append(conflictedFiles, resolver.NonFileConflictFiles...)
	result := BackportCommandResult{
		ConflictResolvedFiles:   resolver.ConflictResolvedFiles,
		ConflictUnresolvedFiles: conflictedFiles,
	}
	if len(conflictedFiles) > 0 {
		return result, fmt.Errorf("conflict found")
	}
	commit := &object.Commit{
		Message:      targetCommit.Message + "\n\nBackported from " + commitHash.String(),
		Author:       targetCommit.Author,
		Committer:    targetCommit.Committer,
		TreeHash:     mergeResult.TreeHash,
		ParentHashes: []plumbing.Hash{currentCommitHash},
	}
	obj := s.storage.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		return result, fmt.Errorf("failed to encode the commit: %v", err)
	}
	newCommitHash, err := s.storage.SetEncodedObject(obj)
	if err != nil {
		return result, fmt.Errorf("failed to set the commit: %v", err)
	}
	result.CommitHash = newCommitHash.String()
	s.newObjects = append(s.newObjects, newCommitHash)
	s.newObjects = append(s.newObjects, mergeResult.NewHashes...)
	s.newObjects = append(s.newObjects, resolver.NewHashes...)
	return result, nil
}

func (s *backport) push(ctx context.Context, commitHash plumbing.Hash) error {
	var buf bytes.Buffer
	packEncoder := packfile.NewEncoder(&buf, s.storage, false)
	if _, err := packEncoder.Encode(s.newObjects, 0); err != nil {
		return nil
	}

	var currentRefHash *plumbing.Hash
	if s.args.CurrentRefHash != "" {
		hash := plumbing.NewHash(s.args.CurrentRefHash)
		currentRefHash = &hash
	}

	pushDebugInfo, err := push.Push(ctx, s.args.RepoURL, s.client, &buf, []push.RefUpdate{
		{
			Name:    plumbing.ReferenceName(s.args.Ref),
			OldHash: currentRefHash,
			NewHash: commitHash,
		},
	})
	s.pushDebugInfo = &pushDebugInfo
	if err != nil {
		return fmt.Errorf("failed to push: %v", err)
	}
	return nil
}
