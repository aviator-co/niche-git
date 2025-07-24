// Copyright 2025 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package e2e_tests

import (
	"testing"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

func TestUpdateRefs(t *testing.T) {
	repo := NewTempRepo(t)

	baseHash := repo.CommitFile(t, "file1", "test")
	output := nichegit.UpdateRefs(
		t.Context(),
		nil,
		nichegit.UpdateRefsArgs{
			RepoURL: "file://" + repo.RepoDir,
			RefUpdateCommands: []nichegit.RefUpdateCommand{
				{
					RefName: "refs/heads/random",
					OldHash: plumbing.ZeroHash.String(),
					NewHash: baseHash.String(),
				},
			},
		},
	)
	require.Empty(t, output.Error)
	require.Contains(t, repo.Git(t, "branch"), "random")
}

func TestUpdateRefs_Conflict_Atomic(t *testing.T) {
	repo := NewTempRepo(t)

	baseHash := repo.CommitFile(t, "file1", "test")
	repo.Git(t, "switch", "--detach", "HEAD")
	output := nichegit.UpdateRefs(
		t.Context(),
		nil,
		nichegit.UpdateRefsArgs{
			RepoURL: "file://" + repo.RepoDir,
			RefUpdateCommands: []nichegit.RefUpdateCommand{
				{
					RefName: "refs/heads/main",
					OldHash: plumbing.ZeroHash.String(),
					NewHash: baseHash.String(),
				},
				{
					RefName: "refs/heads/random",
					OldHash: plumbing.ZeroHash.String(),
					NewHash: baseHash.String(),
				},
			},
		},
	)
	require.Contains(t, output.Error, "atomic transaction failed")
	require.NotContains(t, repo.Git(t, "branch"), "random")
}

func TestUpdateRefs_PreconditionMismatch(t *testing.T) {
	repo := NewTempRepo(t)

	baseHash := repo.CommitFile(t, "file1", "test")
	repo.Git(t, "checkout", "-b", "test")
	anotherHash := repo.CommitFile(t, "file2", "test")

	output := nichegit.UpdateRefs(
		t.Context(),
		nil,
		nichegit.UpdateRefsArgs{
			RepoURL: "file://" + repo.RepoDir,
			RefUpdateCommands: []nichegit.RefUpdateCommand{
				{
					RefName: "refs/heads/main",
					// main is on baseHash. So this should fail the
					// precondition.
					OldHash: anotherHash.String(),
					NewHash: baseHash.String(),
				},
			},
		},
	)
	require.Contains(t, output.Error, "atomic transaction failed")
}
