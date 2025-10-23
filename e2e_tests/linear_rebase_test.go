// Copyright 2025 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package e2e_tests

import (
	"net/http"
	"strings"
	"testing"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/stretchr/testify/require"
)

func TestLinearRebase(t *testing.T) {
	repo := NewTempRepo(t)

	baseHash1 := repo.CommitFile(t, "file", "1")
	repo.Git(t, "checkout", "-b", "branch1")
	repo.CommitFile(t, "file", "2")
	baseHash2 := repo.CommitFile(t, "file", "3")
	repo.Git(t, "checkout", "-b", "branch2")
	repo.CommitFile(t, "file", "4")
	baseHash3 := repo.CommitFile(t, "file", "5")
	repo.Git(t, "checkout", "-b", "branch3")
	repo.CommitFile(t, "file", "6")
	repo.CommitFile(t, "file", "7")
	repo.Git(t, "switch", "main")
	mainHash := repo.CommitFile(t, "unrelated", "unrelated")

	output := nichegit.LinearRebase(
		t.Context(),
		http.DefaultClient,
		nichegit.LinearRebaseArgs{
			RepoURL:           "file://" + repo.RepoDir,
			DestinationCommit: mainHash.String(),
			Refs: []nichegit.LinearRebaseArgRef{
				{
					Ref:        "refs/heads/branch1",
					BaseCommit: baseHash1.String(),
				},
				{
					Ref:        "refs/heads/branch2",
					BaseCommit: baseHash2.String(),
				},
				{
					Ref:        "refs/heads/branch3",
					BaseCommit: baseHash3.String(),
				},
			},
		},
	)
	require.Empty(t, output.Error, "Expected no error during linear rebase")

	repo.Git(t, "switch", "branch1")
	branch1Hash := strings.TrimSpace(repo.Git(t, "rev-parse", "HEAD"))
	require.Equal(t, "3", repo.ReadFile(t, "file"), "Branch1 should have the latest commit after rebase")

	repo.Git(t, "switch", "branch2")
	branch2Hash := strings.TrimSpace(repo.Git(t, "rev-parse", "HEAD"))
	require.Equal(t, "5", repo.ReadFile(t, "file"), "Branch2 should have the latest commit after rebase")

	repo.Git(t, "switch", "branch3")
	branch3Hash := strings.TrimSpace(repo.Git(t, "rev-parse", "HEAD"))
	require.Equal(t, "7", repo.ReadFile(t, "file"), "Branch3 should have the latest commit after rebase")

	require.Equal(t, "unrelated", repo.ReadFile(t, "unrelated"), "Should still have the unrelated commit")

	expected := []*nichegit.LinearRebaseResult{
		{
			Ref:        "refs/heads/branch1",
			CommitHash: branch1Hash,
		},
		{
			Ref:        "refs/heads/branch2",
			CommitHash: branch2Hash,
		},
		{
			Ref:        "refs/heads/branch3",
			CommitHash: branch3Hash,
		},
	}
	require.Equal(t, expected, output.LinearRebaseResults, "Expected linear rebase results to match")
}

func TestLinearRebase_NewCommits(t *testing.T) {
	repo := NewTempRepo(t)

	baseHash1 := repo.CommitFile(t, "file", "1")
	repo.Git(t, "checkout", "-b", "branch1")
	repo.CommitFile(t, "file", "2")
	baseHash2 := repo.CommitFile(t, "file", "3")
	repo.Git(t, "checkout", "-b", "branch2")
	repo.CommitFile(t, "file", "4")
	baseHash3 := repo.CommitFile(t, "file", "5")
	repo.Git(t, "checkout", "-b", "branch3")
	repo.CommitFile(t, "file", "6")
	repo.CommitFile(t, "file", "7")
	repo.Git(t, "switch", "main")
	mainHash := repo.CommitFile(t, "unrelated", "unrelated")

	repo.Git(t, "switch", "branch2")
	repo.CommitFile(t, "new_file", "new_file")

	repo.Git(t, "switch", "main")

	output := nichegit.LinearRebase(
		t.Context(),
		http.DefaultClient,
		nichegit.LinearRebaseArgs{
			RepoURL:           "file://" + repo.RepoDir,
			DestinationCommit: mainHash.String(),
			Refs: []nichegit.LinearRebaseArgRef{
				{
					Ref:        "refs/heads/branch1",
					BaseCommit: baseHash1.String(),
				},
				{
					Ref:        "refs/heads/branch2",
					BaseCommit: baseHash2.String(),
				},
				{
					Ref:        "refs/heads/branch3",
					BaseCommit: baseHash3.String(),
				},
			},
		},
	)
	require.Empty(t, output.Error, "Expected no error during linear rebase")

	repo.Git(t, "switch", "branch1")
	branch1Hash := strings.TrimSpace(repo.Git(t, "rev-parse", "HEAD"))
	require.Equal(t, "3", repo.ReadFile(t, "file"), "Branch1 should have the latest commit after rebase")

	repo.Git(t, "switch", "branch2")
	branch2Hash := strings.TrimSpace(repo.Git(t, "rev-parse", "HEAD"))
	require.Equal(t, "5", repo.ReadFile(t, "file"), "Branch2 should have the latest commit after rebase")
	require.Equal(t, "new_file", repo.ReadFile(t, "new_file"), "Branch2 should have a new file after rebase")

	repo.Git(t, "switch", "branch3")
	branch3Hash := strings.TrimSpace(repo.Git(t, "rev-parse", "HEAD"))
	require.Equal(t, "7", repo.ReadFile(t, "file"), "Branch3 should have the latest commit after rebase")
	require.Equal(t, "new_file", repo.ReadFile(t, "new_file"), "Branch3 should have a new file after rebase")

	require.Equal(t, "unrelated", repo.ReadFile(t, "unrelated"), "Should still have the unrelated commit")

	expected := []*nichegit.LinearRebaseResult{
		{
			Ref:        "refs/heads/branch1",
			CommitHash: branch1Hash,
		},
		{
			Ref:        "refs/heads/branch2",
			CommitHash: branch2Hash,
		},
		{
			Ref:        "refs/heads/branch3",
			CommitHash: branch3Hash,
		},
	}
	require.Equal(t, expected, output.LinearRebaseResults, "Expected linear rebase results to match")
}
