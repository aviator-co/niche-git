// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package e2e_tests

import (
	"testing"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/stretchr/testify/require"
)

func TestBackport_Resolve_Conflict(t *testing.T) {
	repo := NewTempRepo(t)

	repo.CommitFile(t, "file1", `
	def foo():
	    return 1
	
	def bar():
	    return 1

	def baz():
	    return 1
	`)

	repo.Git(t, "checkout", "main", "-b", "feature1")
	feature1Hash := repo.CommitFile(t, "file1", `
	def foo():
	    return 1
	
	def bar():
	    return 2

	def baz():
	    return 1
	`)

	repo.Git(t, "checkout", "main", "-b", "feature2")
	feature2Hash := repo.CommitFile(t, "file1", `
	def foo():
	    return 1
	
	def bar():
	    return 1

	def baz():
	    return 2
	`)

	repo.Git(t, "checkout", "main")
	baseHash2 := repo.CommitFile(t, "file1", `
	def foo():
	    return 2
	
	def bar():
	    return 1

	def baz():
	    return 1
	`)

	output := nichegit.Backport(
		t.Context(),
		nil,
		nichegit.BackportArgs{
			RepoURL:        "file://" + repo.RepoDir,
			BaseCommitHash: baseHash2.String(),
			BackportCommits: []string{
				feature1Hash.String(),
				feature2Hash.String(),
			},
			Ref:            "refs/heads/mq-tmp-branch",
			CurrentRefHash: "",
		},
	)
	require.Empty(t, output.Error)
	require.Len(t, output.CommandResults, 2)

	repo.Git(t, "checkout", "mq-tmp-branch")
	require.Equal(t, `
	def foo():
	    return 2
	
	def bar():
	    return 2

	def baz():
	    return 2
	`+"\n",
		repo.ReadFile(t, "file1"))
}
