// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package e2e_tests

import (
	"net/http"
	"testing"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/stretchr/testify/require"
)

func TestGetBranchingPoint(t *testing.T) {
	repo := NewTempRepo(t)

	repo.CommitFile(t, "file", "1")
	h2 := repo.CommitFile(t, "file", "2")
	repo.Git(t, "checkout", "-b", "branch1")
	repo.CommitFile(t, "file", "3")
	h4 := repo.CommitFile(t, "file", "4")
	repo.Git(t, "switch", "main")
	repo.CommitFile(t, "file", "5")
	h6 := repo.CommitFile(t, "file", "6")

	output := nichegit.GetBranchingPoint(
		t.Context(),
		http.DefaultClient,
		nichegit.GetBranchingPointArgs{
			RepoURL:        "file://" + repo.RepoDir,
			MainRefHash:    h6.String(),
			FeatureRefHash: h4.String(),
		},
	)

	require.Empty(t, output.Error)
	require.Equal(t, output.BranchingPointHash, h2.String(), "Expected to have a branching point")
}
