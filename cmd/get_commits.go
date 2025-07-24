// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/spf13/cobra"
)

var (
	getCommitsArgs       nichegit.GetCommitsArgs
	getCommitsOutputFile string
)

var getCommitsCmd = &cobra.Command{
	Use: "get-commits",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client := &http.Client{Transport: &authnRoundtripper{}}
		output := nichegit.GetCommits(ctx, client, getCommitsArgs)
		if err := writeJSON(getCommitsOutputFile, output); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCommitsCmd)
	getCommitsCmd.Flags().StringVar(&getCommitsArgs.RepoURL, "repo-url", "", "Git repository URL")
	getCommitsCmd.Flags().StringSliceVar(&getCommitsArgs.WantCommitHashes, "want-commit-hashes", nil, "Want commit hashes")
	getCommitsCmd.Flags().StringSliceVar(&getCommitsArgs.HaveCommitHashes, "have-commit-hashes", nil, "Have commit hashes")
	_ = getCommitsCmd.MarkFlagRequired("repo-url")

	getCommitsCmd.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	getCommitsCmd.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	getCommitsCmd.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")

	getCommitsCmd.Flags().StringVar(&getCommitsOutputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
}
