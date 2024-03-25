// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/aviator-co/niche-git/internal/fetch"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

type CommitSignature struct {
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
}

type CommitInfo struct {
	// Hash is the commit hash.
	Hash string `json:"hash"`

	// Author is the author of the commit.
	Author CommitSignature `json:"author"`

	// Committer is the committer of the commit.
	Committer CommitSignature `json:"committer"`

	// Message is the commit message.
	Message string `json:"message"`

	// TreeHash is the hash of the tree object of the commit.
	TreeHash string `json:"treeHash"`

	// ParentHashes are the hashes of the parent commits.
	ParentHashes []string `json:"parentHashes"`
}

type FetchCommitsDebugInfo struct {
	// PackfileSize is the size of the fetched packfile in bytes.
	PackfileSize int

	// ResponseHeaders is the headers of the HTTP response when fetching the packfile.
	ResponseHeaders http.Header
}

func FetchCommits(repoURL string, client *http.Client, wantCommitHashes, haveCommitHashes []string) ([]*CommitInfo, *FetchCommitsDebugInfo, error) {
	packfilebs, headers, err := fetch.FetchCommitOnlyPackfile(repoURL, client, wantCommitHashes, haveCommitHashes)
	debugInfo := &FetchCommitsDebugInfo{
		PackfileSize:    len(packfilebs),
		ResponseHeaders: headers,
	}
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

	var ret []*CommitInfo
	for hash := range storage.Commits {
		commit, err := object.GetCommit(storage, hash)
		if err != nil {
			return nil, debugInfo, fmt.Errorf("cannot parse %q in the fetched packfile: %v", hash, err)
		}
		ret = append(ret, convertCommitInfo(commit))
	}
	return ret, debugInfo, nil
}

func convertCommitInfo(commit *object.Commit) *CommitInfo {
	var parentHashes []string
	for _, parent := range commit.ParentHashes {
		parentHashes = append(parentHashes, parent.String())

	}
	return &CommitInfo{
		Hash: commit.Hash.String(),
		Author: CommitSignature{
			Name:      commit.Author.Name,
			Email:     commit.Author.Email,
			Timestamp: commit.Author.When,
		},
		Committer: CommitSignature{
			Name:      commit.Committer.Name,
			Email:     commit.Committer.Email,
			Timestamp: commit.Committer.When,
		},
		Message:      commit.Message,
		TreeHash:     commit.TreeHash.String(),
		ParentHashes: parentHashes,
	}
}
