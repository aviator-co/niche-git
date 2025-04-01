// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

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

type SquashCommand struct {
	CommitHashStart string `json:"commitHashStart"`
	CommitHashEnd   string `json:"commitHashEnd"`
	CommitMessage   string `json:"commitMessage"`
	Committer       string `json:"committer"`
	CommitterEmail  string `json:"committerEmail"`
	CommitterTime   string `json:"committerTime"`
	Author          string `json:"author"`
	AuthorEmail     string `json:"authorEmail"`
	AuthorTime      string `json:"authorTime"`
}

func (c SquashCommand) committer() (object.Signature, error) {
	t, err := time.Parse(time.RFC3339, c.CommitterTime)
	if err != nil {
		return object.Signature{}, fmt.Errorf("failed to parse the committer time: %v", err)
	}
	return object.Signature{Name: c.Committer, Email: c.CommitterEmail, When: t}, nil
}

func (c SquashCommand) author() (object.Signature, error) {
	t, err := time.Parse(time.RFC3339, c.AuthorTime)
	if err != nil {
		return object.Signature{}, fmt.Errorf("failed to parse the author time: %v", err)
	}
	return object.Signature{Name: c.Author, Email: c.AuthorEmail, When: t}, nil
}

type SquashPushArgs struct {
	RepoURL        string          `json:"repoURL"`
	BaseCommitHash string          `json:"baseCommitHash"`
	SquashCommands []SquashCommand `json:"squashCommands"`
	Ref            string          `json:"ref"`
	CurrentRefHash string          `json:"currentRefHash"`
}

type SquashCommandResult struct {
	CommitHash              string   `json:"commitHash"`
	ConflictResolvedFiles   []string `json:"conflictResolvedFiles"`
	ConflictUnresolvedFiles []string `json:"conflictUnresolvedFiles"`
}

type SquashPushOutput struct {
	CommandResults  []SquashCommandResult   `json:"commandResults"`
	FetchDebugInfos []*debug.FetchDebugInfo `json:"fetchDebugInfos"`
	PushDebugInfo   *debug.PushDebugInfo    `json:"pushDebugInfo"`
	Error           string                  `json:"error,omitempty"`
}

func SquashPush(client *http.Client, args SquashPushArgs) SquashPushOutput {
	s := &squashPush{
		client:  client,
		args:    args,
		storage: memory.NewStorage(),
	}
	err := s.run()
	output := SquashPushOutput{
		CommandResults:  s.commandResults,
		FetchDebugInfos: s.fetchDebugInfos,
		PushDebugInfo:   s.pushDebugInfo,
	}
	if err != nil {
		output.Error = err.Error()
	}
	return output
}

type squashPush struct {
	client          *http.Client
	args            SquashPushArgs
	storage         *memory.Storage
	fetchDebugInfos []*debug.FetchDebugInfo
	pushDebugInfo   *debug.PushDebugInfo
	newObjects      []plumbing.Hash
	commandResults  []SquashCommandResult
}

func (s *squashPush) run() error {
	if err := s.fetchCommits(); err != nil {
		return err
	}
	currentCommitHash := plumbing.NewHash(s.args.BaseCommitHash)
	for _, command := range s.args.SquashCommands {
		commandResult, err := s.squashCommit(currentCommitHash, command)
		s.commandResults = append(s.commandResults, commandResult)
		if err != nil {
			return err
		}
		currentCommitHash = plumbing.NewHash(commandResult.CommitHash)
	}
	if err := s.push(currentCommitHash); err != nil {
		return err
	}
	return nil
}

func (s *squashPush) fetchCommits() error {
	hashes := make(map[plumbing.Hash]bool)
	hashes[plumbing.NewHash(s.args.BaseCommitHash)] = true
	for _, command := range s.args.SquashCommands {
		hashes[plumbing.NewHash(command.CommitHashStart)] = true
		hashes[plumbing.NewHash(command.CommitHashEnd)] = true
	}
	packfilebs, fetchDebugInfo, err := fetch.FetchBlobNonePackfile(s.args.RepoURL, s.client, slices.AppendSeq(make([]plumbing.Hash, 0, len(hashes)), maps.Keys(hashes)))
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

func (s *squashPush) squashCommit(currentCommitHash plumbing.Hash, command SquashCommand) (SquashCommandResult, error) {
	treeDestination, err := s.getTreeFromCommit(currentCommitHash)
	if err != nil {
		return SquashCommandResult{}, err
	}
	treeMergeBase, err := s.getTreeFromCommit(plumbing.NewHash(command.CommitHashStart))
	if err != nil {
		return SquashCommandResult{}, err
	}
	treeNewContent, err := s.getTreeFromCommit(plumbing.NewHash(command.CommitHashEnd))
	if err != nil {
		return SquashCommandResult{}, err
	}

	collector := &conflictBlobCollector{}
	mergeResult, err := merge.MergeTree(s.storage, treeNewContent, treeDestination, treeMergeBase, collector.Resolve)
	if err != nil {
		return SquashCommandResult{}, fmt.Errorf("failed to merge the trees: %v", err)
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
			packfilebs, fetchBlobDebugInfo, err := fetch.FetchBlobPackfile(s.args.RepoURL, s.client, missingBlobHashes)
			s.fetchDebugInfos = append(s.fetchDebugInfos, &fetchBlobDebugInfo)
			if err != nil {
				return SquashCommandResult{}, err
			}
			parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), s.storage)
			if err != nil {
				return SquashCommandResult{}, fmt.Errorf("failed to parse packfile: %v", err)
			}
			if _, err := parser.Parse(); err != nil {
				return SquashCommandResult{}, fmt.Errorf("failed to parse packfile: %v", err)
			}
		}
		mergeResult, err = merge.MergeTree(s.storage, treeNewContent, treeDestination, treeMergeBase, resolver.Resolve)
		if err != nil {
			return SquashCommandResult{}, fmt.Errorf("failed to merge the trees: %v", err)
		}
	}
	var conflictedFiles []string
	conflictedFiles = append(conflictedFiles, resolver.ConflictOpenFiles...)
	conflictedFiles = append(conflictedFiles, resolver.BinaryConflictFiles...)
	conflictedFiles = append(conflictedFiles, resolver.NonFileConflictFiles...)
	result := SquashCommandResult{
		ConflictResolvedFiles:   resolver.ConflictResolvedFiles,
		ConflictUnresolvedFiles: conflictedFiles,
	}
	if len(conflictedFiles) > 0 {
		return result, fmt.Errorf("conflict found")
	}
	author, err := command.author()
	if err != nil {
		return result, err
	}
	committer, err := command.committer()
	if err != nil {
		return result, err
	}
	commit := &object.Commit{
		Message:      command.CommitMessage,
		Author:       author,
		Committer:    committer,
		TreeHash:     mergeResult.TreeHash,
		ParentHashes: []plumbing.Hash{currentCommitHash},
	}
	obj := s.storage.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		return result, fmt.Errorf("failed to encode the commit: %v", err)
	}
	commitHash, err := s.storage.SetEncodedObject(obj)
	if err != nil {
		return result, fmt.Errorf("failed to set the commit: %v", err)
	}
	result.CommitHash = commitHash.String()
	s.newObjects = append(s.newObjects, commitHash)
	s.newObjects = append(s.newObjects, mergeResult.NewHashes...)
	s.newObjects = append(s.newObjects, resolver.NewHashes...)
	return result, nil
}

func (s *squashPush) push(commitHash plumbing.Hash) error {
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

	pushDebugInfo, err := push.Push(s.args.RepoURL, s.client, &buf, []push.RefUpdate{
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

func (s *squashPush) getTreeFromCommit(commitHash plumbing.Hash) (*object.Tree, error) {
	commit, err := object.GetCommit(s.storage, commitHash)
	if err != nil {
		return nil, fmt.Errorf("cannot find %q in the fetched packfile: %v", commitHash.String(), err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("cannot find the tree of %q in the fetched packfile: %v", commitHash.String(), err)
	}
	return tree, nil
}
