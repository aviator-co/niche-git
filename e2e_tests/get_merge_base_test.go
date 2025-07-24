// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package e2e_tests

import (
	"net/http"
	"sort"
	"testing"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/stretchr/testify/require"
)

func TestGetMergeBase(t *testing.T) {
	repo := NewTempRepo(t)

	repo.CommitFile(t, "file", "1")
	h2 := repo.CommitFile(t, "file", "2")
	repo.Git(t, "checkout", "-b", "branch1")
	repo.CommitFile(t, "file", "3")
	h4 := repo.CommitFile(t, "file", "4")
	repo.Git(t, "switch", "main")
	repo.CommitFile(t, "file", "5")
	h6 := repo.CommitFile(t, "file", "6")

	output := nichegit.GetMergeBase(
		http.DefaultClient,
		nichegit.GetMergeBaseArgs{
			RepoURL: "file://" + repo.RepoDir,
			CommitHashes: []string{
				h6.String(),
				h4.String(),
			},
		},
	)
	expected := []nichegit.FoundMergeBase{
		{
			CommitHash: h2.String(),
			Generation: 3,
		},
	}
	sortFoundMergeBases(output.MergeBases)
	sortFoundMergeBases(expected)

	require.Empty(t, output.Error)
	require.Equal(t, expected, output.MergeBases, "Expected merge base not found")
}

func TestGetMergeBase_Merge(t *testing.T) {
	repo := NewTempRepo(t)

	repo.CommitFile(t, "file", "1")
	repo.CommitFile(t, "file", "2")
	repo.Git(t, "checkout", "-b", "branch1")
	h3 := repo.CommitFile(t, "file", "3")
	repo.CommitFile(t, "file", "4")
	repo.Git(t, "switch", "main")
	h5 := repo.CommitFile(t, "file", "5")
	repo.Git(t, "merge", h3.String())
	h6 := repo.CommitFile(t, "file", "6")
	repo.Git(t, "switch", "branch1")
	repo.CommitFile(t, "file", "7")
	repo.Git(t, "merge", h5.String())
	h8 := repo.CommitFile(t, "file", "8")

	output := nichegit.GetMergeBase(
		http.DefaultClient,
		nichegit.GetMergeBaseArgs{
			RepoURL: "file://" + repo.RepoDir,
			CommitHashes: []string{
				h6.String(),
				h8.String(),
			},
		},
	)
	expected := []nichegit.FoundMergeBase{
		{
			CommitHash: h5.String(),
			Generation: 4,
		},
		{
			CommitHash: h3.String(),
			Generation: 4,
		},
	}
	sortFoundMergeBases(output.MergeBases)
	sortFoundMergeBases(expected)

	require.Empty(t, output.Error)
	require.Equal(t, expected, output.MergeBases, "Expected merge base not found")
}

func sortFoundMergeBases(mergeBases []nichegit.FoundMergeBase) {
	sort.Slice(mergeBases, func(i, j int) bool {
		if mergeBases[i].Generation != mergeBases[j].Generation {
			return mergeBases[i].Generation < mergeBases[j].Generation
		}
		return mergeBases[i].CommitHash < mergeBases[j].CommitHash
	})
}
