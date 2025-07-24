// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package e2e_tests

import (
	"testing"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/aviator-co/niche-git/cmd"
	"github.com/stretchr/testify/require"
)

func TestSquashCherryPick_Resolve_Conflict(t *testing.T) {
	repo := NewTempRepo(t)

	baseHash := repo.CommitFile(t, "file1", `
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

	output := cmd.SquashPush(
		t.Context(),
		nichegit.SquashPushArgs{
			RepoURL:        "file://" + repo.RepoDir,
			BaseCommitHash: baseHash2.String(),
			SquashCommands: []nichegit.SquashCommand{
				{
					CommitHashStart: baseHash.String(),
					CommitHashEnd:   feature1Hash.String(),
					CommitMessage:   "feature1",
					Committer:       "aviator-bot",
					CommitterEmail:  "aviator-bot@nonexistent",
					CommitterTime:   "2024-08-22T00:00:00Z",
					Author:          "aviator-bot",
					AuthorEmail:     "aviator-bot@nonexistent",
					AuthorTime:      "2024-08-22T00:00:00Z",
				},
				{
					CommitHashStart: baseHash.String(),
					CommitHashEnd:   feature2Hash.String(),
					CommitMessage:   "feature2",
					Committer:       "aviator-bot",
					CommitterEmail:  "aviator-bot@nonexistent",
					CommitterTime:   "2024-08-22T00:00:00Z",
					Author:          "aviator-bot",
					AuthorEmail:     "aviator-bot@nonexistent",
					AuthorTime:      "2024-08-22T00:00:00Z",
				},
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
