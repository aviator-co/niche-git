// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package e2e_tests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSquashCherryPick_Conflict_Resolve(t *testing.T) {
	repo := NewTempRepo(t)

	baseHash := repo.CommitFile(t, "file1", `
line 1
line 2
line 3
line 4
line 6
line 7
line 8
line 9
`)

	repo.Git(t, "checkout", "-b", "feature")
	featureHash := repo.CommitFile(t, "file1", `
line 1
line 2
line 3
line 4
line 6
line 7
line 8
line 9
line 10
`)
	repo.Git(t, "checkout", "main")
	targetHash := repo.CommitFile(t, "file1", `
line 1
line 2
line 3
line 4
line 5
line 6
line 7
line 8
line 9
`)

	// At this point, featureHash adds line 10, targetHash adds line 5.
	RequireNicheGit(t,
		"squash-cherry-pick",
		"--repo-url", "file://"+repo.RepoDir,
		"--cherry-pick-from", featureHash.String(),
		"--cherry-pick-to", targetHash.String(),
		"--cherry-pick-base", baseHash.String(),
		"--commit-message", "pick feature",
		"--author", "cherry-picker",
		"--author-email", "cherry-picker@nonexistent",
		"--committer", "cherry-picker",
		"--committer-email", "cherry-picker@nonexistent",
		"--ref", "refs/heads/cherry-pick-result",
	)
	repo.Git(t, "checkout", "cherry-pick-result")
	require.Equal(t, `
line 1
line 2
line 3
line 4
line 5
line 6
line 7
line 8
line 9
line 10
`, repo.ReadFile(t, "file1"))
}

func TestSquashCherryPick_Conflict_Unresolved(t *testing.T) {
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

	RequireNicheGit(t,
		"squash-cherry-pick",
		"--repo-url", "file://"+repo.RepoDir,
		"--cherry-pick-from", featureHash.String(),
		"--cherry-pick-to", targetHash.String(),
		"--cherry-pick-base", baseHash.String(),
		"--commit-message", "pick feature",
		"--author", "cherry-picker",
		"--author-email", "cherry-picker@nonexistent",
		"--committer", "cherry-picker",
		"--committer-email", "cherry-picker@nonexistent",
		"--ref", "refs/heads/cherry-pick-result",
		"--conflict-ref", "refs/heads/cherry-pick-conflict",
	)
	repo.Git(t, "checkout", "cherry-pick-conflict")
	require.Equal(t, `
line 1
<<<<<<<<< Cherry-pick content
line 5
line 6
=========
line 8
line 9
>>>>>>>>> Base content
`, repo.ReadFile(t, "file1"))
}
