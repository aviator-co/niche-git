// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package e2e_tests

import (
	"testing"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/aviator-co/niche-git/cmd"
	"github.com/stretchr/testify/require"
)

func TestModifiedFilesRegexpMatches(t *testing.T) {
	repo := NewTempRepo(t)

	repo.CommitFile(t, "tests/com/example/ServiceTest.java", "")
	baseHash := repo.CommitFile(t, "tests/com/example/E2ETest.java", `
	@Ignore
	public void test1() { }
	@Ignore
	public void test2() { }
	@Ignore
	public void test3() { }
	`)

	repo.CommitFile(t, "tests/com/example/ServiceTest.java", "random change")
	repo.CommitFile(t, "tests/com/example/E2ETest.java", `
	@Ignore
	public void test1() { }
	@Ignore
	public void test2() { }
	@Ignore
	public void test3() { }
	@Ignore
	public void test4() { }
	@Ignore
	public void test5() { }
	`)
	targetHash := repo.CommitFile(t, "src/migrations/add_table_20240822.sql", "")

	output := cmd.GetModifiedFilesRegexpMatches(
		cmd.GetModifiedFilesRegexpMatchesArgs{
			RepoURL:     "file://" + repo.RepoDir,
			CommitHash1: baseHash.String(),
			CommitHash2: targetHash.String(),
			Patterns: map[string]cmd.GetModifiedFilesPattern{
				"ignore_test": {
					FilePathPatterns:   []string{"**/*.java"},
					FileContentPattern: `@Ignore`,
				},
				"database_migration": {
					FilePathPatterns: []string{"src/migrations/*.sql"},
				},
			},
		},
	)
	require.Empty(t, output.Error)
	require.Equal(t, []*nichegit.ModifiedFile{
		{
			Path:   "src/migrations/add_table_20240822.sql",
			Status: nichegit.ModificationStatusAdded,
			Matches: map[string]*nichegit.ModifiedFilePatternMatch{
				"database_migration": {
					Before: 0,
					After:  1,
				},
			},
		},
		{
			Path:   "tests/com/example/E2ETest.java",
			Status: nichegit.ModificationStatusModified,
			Matches: map[string]*nichegit.ModifiedFilePatternMatch{
				"ignore_test": {
					Before: 3,
					After:  5,
				},
			},
		},
		{
			Path:   "tests/com/example/ServiceTest.java",
			Status: nichegit.ModificationStatusModified,
		},
	}, output.Files)
}
