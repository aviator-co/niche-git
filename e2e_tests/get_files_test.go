// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package e2e_tests

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

func TestGetFiles(t *testing.T) {
	repo := NewTempRepo(t)
	h := repo.CommitFile(t, "styleguide.md", "# Style Guide\n")

	output := nichegit.GetFiles(t.Context(), http.DefaultClient, nichegit.GetFilesArgs{
		RepoURL:    "file://" + repo.RepoDir,
		CommitHash: h.String(),
		FilePaths:  []string{"styleguide.md", "nonexistent.md"},
	})

	require.Empty(t, output.Error)
	require.Equal(t, "# Style Guide\n", output.Files["styleguide.md"])
	require.NotContains(t, output.Files, "nonexistent.md")
}

// A symlinked doc used to read as a missing file, which made a repo whose
// AGENTS.md points at its style guide indistinguishable from a repo with no
// docs at all.
func TestGetFiles_Symlinks(t *testing.T) {
	repo := NewTempRepo(t)
	repo.CommitFile(t, "styleguide.md", "# Style Guide\n")
	repo.CommitFile(t, "docs/conventions.md", "# Conventions\n")

	symlink := func(target, linkName string) {
		t.Helper()
		require.NoError(t, os.Symlink(target, filepath.Join(repo.RepoDir, linkName)))
	}
	symlink("styleguide.md", "agents.md")         // same directory
	symlink("../styleguide.md", "docs/AGENTS.md") // relative, walks up
	symlink("nonexistent.md", "dangling.md")      // target not in the tree
	symlink("agents.md", "chained.md")            // symlink to a symlink
	symlink("/etc/passwd", "absolute.md")         // escapes the repository
	repo.Git(t, "add", "-A")
	repo.Git(t, "commit", "-m", "Add symlinks")
	h := plumbing.NewHash(strings.TrimSpace(repo.Git(t, "rev-parse", "HEAD")))

	output := nichegit.GetFiles(t.Context(), http.DefaultClient, nichegit.GetFilesArgs{
		RepoURL:    "file://" + repo.RepoDir,
		CommitHash: h.String(),
		FilePaths: []string{
			"agents.md", "docs/AGENTS.md", "styleguide.md",
			"dangling.md", "chained.md", "absolute.md",
		},
	})

	require.Empty(t, output.Error)

	// A symlink yields the content of what it points at, keyed by the path
	// that was actually requested.
	require.Equal(t, "# Style Guide\n", output.Files["agents.md"])
	require.Equal(t, "# Style Guide\n", output.Files["docs/AGENTS.md"])

	// Regular files are unaffected.
	require.Equal(t, "# Style Guide\n", output.Files["styleguide.md"])

	// Nothing readable behind these, and none of them is an error.
	require.NotContains(t, output.Files, "dangling.md")
	require.NotContains(t, output.Files, "chained.md")
	require.NotContains(t, output.Files, "absolute.md")
}
