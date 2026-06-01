// Copyright 2026 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package e2e_tests

import (
	"net/http"
	"testing"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/stretchr/testify/require"
)

func TestMergeTree_Conflict(t *testing.T) {
	repo := NewTempRepo(t)

	baseHash := repo.CommitFile(t, "file1", `
line 1
line 2
line 3
`)

	repo.Git(t, "checkout", "-b", "feature")
	featureHash := repo.CommitFile(t, "file1", `
line 1
line 5
line 6
`)
	repo.Git(t, "checkout", "main")
	targetHash := repo.CommitFile(t, "file1", `
line 1
line 8
line 9
`)

	// feature and main both rewrite the same lines of file1 starting from the
	// same base, so the three-way merge must report file1 as a conflict.
	output := nichegit.MergeTree(
		t.Context(),
		http.DefaultClient,
		nichegit.MergeTreeArgs{
			RepoURL:   "file://" + repo.RepoDir,
			Commit1:   featureHash.String(),
			Commit2:   targetHash.String(),
			MergeBase: baseHash.String(),
		},
	)
	require.Empty(t, output.Error)
	require.Contains(t, output.ConflictOpenFiles, "file1")
}

func TestMergeTree_NoConflict(t *testing.T) {
	repo := NewTempRepo(t)

	baseHash := repo.CommitFile(t, "file1", "line 1\n")

	repo.Git(t, "checkout", "-b", "feature")
	featureHash := repo.CommitFile(t, "file2", "feature only\n")
	repo.Git(t, "checkout", "main")
	targetHash := repo.CommitFile(t, "file3", "target only\n")

	// feature adds file2, main adds file3 — disjoint changes, so no conflict.
	output := nichegit.MergeTree(
		t.Context(),
		http.DefaultClient,
		nichegit.MergeTreeArgs{
			RepoURL:   "file://" + repo.RepoDir,
			Commit1:   featureHash.String(),
			Commit2:   targetHash.String(),
			MergeBase: baseHash.String(),
		},
	)
	require.Empty(t, output.Error)
	require.Empty(t, output.ConflictOpenFiles)
	require.Empty(t, output.BinaryConflictFiles)
	require.Empty(t, output.NonFileConflictFiles)
}
