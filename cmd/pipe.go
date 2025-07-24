// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/aviator-co/niche-git/gitprotocontext"
	"github.com/spf13/cobra"
)

var (
	gitFetchRetryCount int
	gitFetchTimeout    time.Duration
	gitPushTimeout     time.Duration
)

var pipeArg struct {
	command    string
	inputFile  string
	outputFile string
}

var pipeCmd = &cobra.Command{
	Use: "pipe",
	RunE: func(cmd *cobra.Command, args []string) error {
		var in io.Reader
		if pipeArg.inputFile == "-" {
			in = os.Stdin
		} else {
			file, err := os.Open(pipeArg.inputFile)
			if err != nil {
				return err
			}
			defer file.Close()
			in = file
		}
		dec := json.NewDecoder(in)

		ctx := cmd.Context()
		ctx = gitprotocontext.WithGitFetchRetryCount(ctx, gitFetchRetryCount)
		ctx = gitprotocontext.WithGitFetchTimeout(ctx, gitFetchTimeout)
		ctx = gitprotocontext.WithGitPushTimeout(ctx, gitPushTimeout)
		client := &http.Client{Transport: &authnRoundtripper{}}

		switch pipeArg.command {
		case "get-commits":
			args := nichegit.GetCommitsArgs{}
			if err := dec.Decode(&args); err != nil {
				return err
			}
			output := nichegit.GetCommits(ctx, client, args)
			return writeJSON(pipeArg.outputFile, output)
		case "get-files":
			args := nichegit.GetFilesArgs{}
			if err := dec.Decode(&args); err != nil {
				return err
			}
			output := nichegit.GetFiles(ctx, client, args)
			return writeJSON(pipeArg.outputFile, output)
		case "get-merge-base":
			args := nichegit.GetMergeBaseArgs{}
			if err := dec.Decode(&args); err != nil {
				return err
			}
			output := nichegit.GetMergeBase(ctx, client, args)
			return writeJSON(pipeArg.outputFile, output)
		case "get-modified-files":
			args := nichegit.GetModifiedFilesArgs{}
			if err := dec.Decode(&args); err != nil {
				return err
			}
			output := nichegit.GetModifiedFiles(ctx, client, args)
			return writeJSON(pipeArg.outputFile, output)
		case "get-modified-files-regexp-matches":
			args := nichegit.GetModifiedFilesRegexpMatchesArgs{}
			if err := dec.Decode(&args); err != nil {
				return err
			}
			output := nichegit.GetModifiedFilesRegexpMatches(ctx, client, args)
			return writeJSON(pipeArg.outputFile, output)
		case "ls-refs":
			args := nichegit.LsRefsArgs{}
			if err := dec.Decode(&args); err != nil {
				return err
			}
			output := nichegit.LsRefs(ctx, client, args)
			return writeJSON(pipeArg.outputFile, output)
		case "squash-cherry-pick":
			args := nichegit.SquashCherryPickArgs{}
			if err := dec.Decode(&args); err != nil {
				return err
			}
			output := nichegit.SquashCherryPick(ctx, client, args)
			return writeJSON(pipeArg.outputFile, output)
		case "squash-push":
			args := nichegit.SquashPushArgs{}
			if err := dec.Decode(&args); err != nil {
				return err
			}
			output := nichegit.SquashPush(ctx, client, args)
			return writeJSON(pipeArg.outputFile, output)
		case "update-refs":
			args := nichegit.UpdateRefsArgs{}
			if err := dec.Decode(&args); err != nil {
				return err
			}
			output := nichegit.UpdateRefs(ctx, client, args)
			return writeJSON(pipeArg.outputFile, output)
		case "backport":
			args := nichegit.BackportArgs{}
			if err := dec.Decode(&args); err != nil {
				return err
			}
			output := nichegit.Backport(ctx, client, args)
			return writeJSON(pipeArg.outputFile, output)
		}

		return fmt.Errorf("unknown command: %s", pipeArg.command)
	},
}

func init() {
	rootCmd.AddCommand(pipeCmd)

	pipeCmd.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	pipeCmd.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	pipeCmd.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")
	pipeCmd.Flags().IntVar(&gitFetchRetryCount, "git-fetch-retry-count", 0, "Number of retries for git fetch operations")
	pipeCmd.Flags().DurationVar(&gitFetchTimeout, "git-fetch-timeout", 0, "Timeout for git fetch operations")
	pipeCmd.Flags().DurationVar(&gitPushTimeout, "git-push-timeout", 0, "Timeout for git push operations")

	pipeCmd.Flags().StringVar(&pipeArg.command, "command", "", "Command to execute")
	pipeCmd.Flags().StringVar(&pipeArg.inputFile, "args-file", "-", "Optional args file path. '-', which is the default, means stdin")
	pipeCmd.Flags().StringVar(&pipeArg.outputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
	_ = pipeCmd.MarkFlagRequired("command")
}
