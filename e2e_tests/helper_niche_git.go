// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package e2e_tests

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kr/text"
	"github.com/stretchr/testify/require"
)

var nicheGitCmdPath string

func init() {
	cmd := exec.Command("go", "build", "../cmd/niche-git")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		panic(err)
	}
	var err error
	nicheGitCmdPath, err = filepath.Abs("./niche-git")
	if err != nil {
		panic(err)
	}
}

type NicheGitOutput struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

func cmdInternal(t *testing.T, exe string, args ...string) NicheGitOutput {
	t.Helper()
	cmd := exec.Command(exe, args...)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	var exitError *exec.ExitError
	if err != nil && !errors.As(err, &exitError) {
		t.Fatal(err)
	}

	output := NicheGitOutput{
		ExitCode: cmd.ProcessState.ExitCode(),
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
	t.Logf("Running niche-git\n"+
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
	return output
}

func NicheGit(t *testing.T, args ...string) NicheGitOutput {
	t.Helper()
	return cmdInternal(t, nicheGitCmdPath, args...)
}

func RequireNicheGit(t *testing.T, args ...string) NicheGitOutput {
	t.Helper()
	output := NicheGit(t, args...)
	require.Equal(t, 0, output.ExitCode, "niche-git %s: exited with %v", args, output.ExitCode)
	return output
}
