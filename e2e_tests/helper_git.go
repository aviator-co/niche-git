// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package e2e_tests

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/kr/text"
	"github.com/stretchr/testify/require"
)

func NewTempRepo(t *testing.T) *GitTestRepo {
	t.Helper()
	dir := t.TempDir()
	init := exec.Command("git", "init", "--initial-branch=main")
	init.Dir = dir
	err := init.Run()
	require.NoError(t, err, "failed to initialize git repository")
	repo := &GitTestRepo{dir}
	require.NoError(t, err, "failed to open repo")

	settings := map[string]string{
		"user.name":  "niche-git-test",
		"user.email": "niche-git-test@nonexistent",
	}
	for k, v := range settings {
		repo.Git(t, "config", k, v)
	}

	repo.CommitFile(t, "README.md", "Hello World")
	return repo
}

type GitTestRepo struct {
	RepoDir string
}

func (r *GitTestRepo) Git(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = r.RepoDir
	err := cmd.Run()
	var exitError *exec.ExitError
	if err != nil && !errors.As(err, &exitError) {
		t.Fatal(err)
	}
	t.Logf("Running git\n"+
		"args: %v\n"+
		"exit code: %v\n"+
		"stdout:\n"+
		"%s"+
		"stderr:\n"+
		"%s",
		args,
		cmd.ProcessState.ExitCode(),
		text.Indent(stdout.String(), "  "),
		text.Indent(stderr.String(), "  "),
	)
	return stdout.String()
}

func (r *GitTestRepo) AddFile(t *testing.T, fp string) {
	t.Helper()
	r.Git(t, "add", fp)
}

func (r *GitTestRepo) CreateFile(t *testing.T, filename string, body string) string {
	t.Helper()
	fp := filepath.Join(r.RepoDir, filename)
	err := os.MkdirAll(filepath.Dir(fp), 0o755)
	require.NoError(t, err, "failed to create dirs: %s", filename)
	err = os.WriteFile(fp, []byte(body), 0o600)
	require.NoError(t, err, "failed to write file: %s", filename)
	return fp
}

func (r *GitTestRepo) CommitFile(t *testing.T, filename string, body string) plumbing.Hash {
	t.Helper()
	filepath := r.CreateFile(t, filename, body)
	r.AddFile(t, filepath)

	args := []string{"commit", "-m", fmt.Sprintf("Write %s", filename)}
	r.Git(t, args...)
	return plumbing.NewHash(strings.TrimSpace(r.Git(t, "rev-parse", "HEAD")))
}

func (r *GitTestRepo) ReadFile(t *testing.T, filename string) string {
	t.Helper()
	fp := filepath.Join(r.RepoDir, filename)
	bs, err := os.ReadFile(fp)
	require.NoError(t, err, "failed to read file: %s", filename)
	return string(bs)
}
