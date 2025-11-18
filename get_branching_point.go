// Copyright 2025 Aviator Technologies, Inc.
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
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

var errNeedMoreCommits = errors.New("need to fetch more commits")

type GetBranchingPointArgs struct {
	RepoURL        string `json:"repoURL"`
	MainRefHash    string `json:"mainRefHash"`
	FeatureRefHash string `json:"featureHash"`
	// InitialDepth is the depth that the command should start fetching the
	// commits. It keeps growning the depth until it hits the bottom of the
	// history. If unspecified, start from 100 commits. If the caller wants
	// to start by taking all the commits, specify -1.
	InitialDepth int `json:"initialDepth"`
}

type GetBranchingPointOutput struct {
	BranchingPointHash string                  `json:"branchingPointHash"`
	FetchDebugInfos    []*debug.FetchDebugInfo `json:"fetchDebugInfos,omitempty"`
	Error              string                  `json:"error,omitempty"`
}

type getBranchingPoint struct {
	client         *http.Client
	repoURL        string
	mainRefHash    plumbing.Hash
	featureRefHash plumbing.Hash
	fetchDepth     int

	storage *memory.Storage

	fetchDebugInfos    []*debug.FetchDebugInfo
	branchingPointHash plumbing.Hash
}

func GetBranchingPoint(ctx context.Context, client *http.Client, args GetBranchingPointArgs) GetBranchingPointOutput {
	gbp := &getBranchingPoint{
		client:         client,
		repoURL:        args.RepoURL,
		mainRefHash:    plumbing.NewHash(args.MainRefHash),
		featureRefHash: plumbing.NewHash(args.FeatureRefHash),
		fetchDepth:     args.InitialDepth,
	}
	if gbp.fetchDepth == 0 {
		gbp.fetchDepth = 100
	}
	err := gbp.run(ctx)
	output := GetBranchingPointOutput{
		BranchingPointHash: gbp.branchingPointHash.String(),
		FetchDebugInfos:    gbp.fetchDebugInfos,
	}
	if err != nil {
		output.Error = err.Error()
	}
	return output
}

func (gbp *getBranchingPoint) run(ctx context.Context) error {
	for {
		if err := gbp.fetch(ctx); err != nil {
			return err
		}
		if err := gbp.findBranchingPoint(); err != nil {
			if errors.Is(err, errNeedMoreCommits) {
				gbp.fetchDepth *= 2
				continue
			}
			return err
		}
		return nil
	}
}

func (gbp *getBranchingPoint) fetch(ctx context.Context) error {
	packfilebs, fetchDebugInfo, err := fetch.FetchCommitOnlyPackfile(
		ctx,
		gbp.repoURL,
		gbp.client,
		[]plumbing.Hash{gbp.mainRefHash, gbp.featureRefHash},
		nil,
		gbp.fetchDepth,
	)
	gbp.fetchDebugInfos = append(gbp.fetchDebugInfos, &fetchDebugInfo)
	if err != nil {
		return err
	}
	gbp.storage = memory.NewStorage()
	parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), gbp.storage)
	if err != nil {
		return fmt.Errorf("failed to parse packfile: %v", err)
	}
	if _, err := parser.Parse(); err != nil {
		return fmt.Errorf("failed to parse packfile: %v", err)
	}
	return nil
}

func (gbp *getBranchingPoint) findBranchingPoint() error {
	mainReachables, err := gbp.findReachableCommits(gbp.mainRefHash)
	if err != nil {
		return err
	}
	currentHash := gbp.featureRefHash
	for {
		commit, err := object.GetCommit(gbp.storage, currentHash)
		if err != nil {
			if errors.Is(err, plumbing.ErrObjectNotFound) {
				// Reached the shallow clone boundary.
				return errNeedMoreCommits
			}
			return err
		}
		if commit.NumParents() == 0 {
			// Reached the beginning of the history.
			return errors.New("feature branch has an independent history")
		}
		if commit.NumParents() > 1 {
			// We only support linear history.
			return errors.New("feature branch has a merge commit")
		}
		nextHash := commit.ParentHashes[0]
		if _, found := mainReachables[nextHash]; found {
			// We found the branching point.
			gbp.branchingPointHash = nextHash
			return nil
		}
		currentHash = nextHash
	}
}

func (gbp *getBranchingPoint) findReachableCommits(startHash plumbing.Hash) (map[plumbing.Hash]any, error) {
	reachables := make(map[plumbing.Hash]any)
	queue := []plumbing.Hash{startHash}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, exists := reachables[current]; !exists {
			reachables[current] = struct{}{}
			commit, err := object.GetCommit(gbp.storage, current)
			if err != nil {
				if errors.Is(err, plumbing.ErrObjectNotFound) {
					// Reached the shallow clone boundary.
					continue
				}
				return nil, fmt.Errorf("failed to get commit %s: %v", current, err)
			}
			queue = append(queue, commit.ParentHashes...)
		}
	}
	return reachables, nil
}
