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
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

type GetMergeBaseArgs struct {
	RepoURL      string   `json:"repoURL"`
	CommitHashes []string `json:"commitHashes"`
}

type FoundMergeBase struct {
	CommitHash string `json:"commitHash"`
	Generation int    `json:"generation"`
}

type GetMergeBaseOutput struct {
	MergeBases      []FoundMergeBase        `json:"mergeBases"`
	FetchDebugInfos []*debug.FetchDebugInfo `json:"fetchDebugInfos"`
	Error           string                  `json:"error,omitempty"`
}

type getMergeBase struct {
	client            *http.Client
	repoURL           string
	commitHashes      []plumbing.Hash
	storage           *memory.Storage
	fetchDebugInfos   []*debug.FetchDebugInfo
	generationNumbers map[plumbing.Hash]int

	mergeBases []FoundMergeBase
}

func GetMergeBase(ctx context.Context, client *http.Client, args GetMergeBaseArgs) GetMergeBaseOutput {
	commitHashes := make([]plumbing.Hash, len(args.CommitHashes))
	for i, hash := range args.CommitHashes {
		commitHashes[i] = plumbing.NewHash(hash)
	}

	gmb := &getMergeBase{
		client:       client,
		repoURL:      args.RepoURL,
		commitHashes: commitHashes,
		storage:      memory.NewStorage(),
	}
	err := gmb.run(ctx)
	output := GetMergeBaseOutput{
		MergeBases:      gmb.mergeBases,
		FetchDebugInfos: gmb.fetchDebugInfos,
	}
	if err != nil {
		output.Error = err.Error()
	}
	return output
}

func (gmb *getMergeBase) run(ctx context.Context) error {
	if err := gmb.fetch(ctx); err != nil {
		return err
	}
	if err := gmb.setGenerationNumbers(); err != nil {
		return err
	}
	if err := gmb.findMergeBases(); err != nil {
		return err
	}
	return nil
}

func (gmb *getMergeBase) fetch(ctx context.Context) error {
	packfilebs, fetchDebugInfo, err := fetch.FetchCommitOnlyPackfile(
		ctx,
		gmb.repoURL,
		gmb.client,
		gmb.commitHashes,
		nil,
	)
	gmb.fetchDebugInfos = append(gmb.fetchDebugInfos, &fetchDebugInfo)
	if err != nil {
		return err
	}

	parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), gmb.storage)
	if err != nil {
		return fmt.Errorf("failed to parse packfile: %v", err)
	}
	if _, err := parser.Parse(); err != nil {
		return fmt.Errorf("failed to parse packfile: %v", err)
	}
	return nil
}

func (gmb *getMergeBase) setGenerationNumbers() error {
	// This function calculates the generation numbers for each commit in the repository.
	// For the definition of the generation numbers, see https://git-scm.com/docs/commit-graph-format/2.19.0
	numParents := make(map[plumbing.Hash]int)
	parentToChildren := make(map[plumbing.Hash][]plumbing.Hash)
	var firstGenerationHashes []plumbing.Hash
	for hash := range gmb.storage.Commits {
		commit, err := object.GetCommit(gmb.storage, hash)
		if err != nil {
			return fmt.Errorf("failed to get commit %s: %v", hash, err)
		}
		for _, parentHash := range commit.ParentHashes {
			parentToChildren[parentHash] = append(parentToChildren[parentHash], hash)
		}
		numParents[hash] = len(commit.ParentHashes)
		if len(commit.ParentHashes) == 0 {
			firstGenerationHashes = append(firstGenerationHashes, hash)
		}
	}

	gmb.generationNumbers = make(map[plumbing.Hash]int)
	for _, hash := range firstGenerationHashes {
		gmb.generationNumbers[hash] = 1
	}
	queue := firstGenerationHashes
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		nextGen := gmb.generationNumbers[current] + 1
		for _, child := range parentToChildren[current] {
			if _, exists := gmb.generationNumbers[child]; !exists {
				gmb.generationNumbers[child] = nextGen
			} else if gmb.generationNumbers[child] < nextGen {
				gmb.generationNumbers[child] = nextGen
			}
			numParents[child]--
			if numParents[child] == 0 {
				// All parents put their generation numbers, so we can add this
				// child to the queue.
				queue = append(queue, child)
			}
		}
	}
	return nil
}

func (gmb *getMergeBase) findMergeBases() error {
	commonlyReachables, err := gmb.findReachableCommits(gmb.commitHashes[0])
	if err != nil {
		return fmt.Errorf("failed to find reachable commits from %s: %v", gmb.commitHashes[0], err)
	}

	for _, startHash := range gmb.commitHashes[1:] {
		reachables, err := gmb.findReachableCommits(startHash)
		if err != nil {
			return fmt.Errorf("failed to find reachable commits from %s: %v", startHash, err)
		}
		for hash := range reachables {
			if _, exists := commonlyReachables[hash]; !exists {
				delete(commonlyReachables, hash)
			}
		}
		for hash := range commonlyReachables {
			if _, exists := reachables[hash]; !exists {
				delete(commonlyReachables, hash)
			}
		}
	}

	// Find the leaves.
	queue := make([]plumbing.Hash, 0, len(commonlyReachables))
	for hash := range commonlyReachables {
		queue = append(queue, hash)
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, exists := commonlyReachables[current]; !exists {
			continue
		}
		reachables, err := gmb.findReachableCommits(current)
		if err != nil {
			return fmt.Errorf("failed to find reachable commits from %s: %v", current, err)
		}
		for hash := range reachables {
			if hash != current {
				delete(commonlyReachables, hash)
			}
		}
	}

	for hash := range commonlyReachables {
		generation := gmb.generationNumbers[hash]
		gmb.mergeBases = append(gmb.mergeBases, FoundMergeBase{
			CommitHash: hash.String(),
			Generation: generation,
		})
	}
	return nil
}

func (gmb *getMergeBase) findReachableCommits(startHash plumbing.Hash) (map[plumbing.Hash]bool, error) {
	reachables := make(map[plumbing.Hash]bool)
	queue := []plumbing.Hash{startHash}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, exists := reachables[current]; !exists {
			reachables[current] = true
			commit, err := object.GetCommit(gmb.storage, current)
			if err != nil {
				return nil, fmt.Errorf("failed to get commit %s: %v", current, err)
			}
			queue = append(queue, commit.ParentHashes...)
		}
	}
	return reachables, nil
}
