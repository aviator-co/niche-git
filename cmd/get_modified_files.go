// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/spf13/cobra"
)

var (
	getModifiedFilesArgs       nichegit.GetModifiedFilesArgs
	getModifiedFilesOutputFile string
)

var getModifiedFilesCmd = &cobra.Command{
	Use: "get-modified-files",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client := &http.Client{Transport: &authnRoundtripper{}}
		output := nichegit.GetModifiedFiles(ctx, client, getModifiedFilesArgs)
		if err := writeJSON(getModifiedFilesOutputFile, output); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getModifiedFilesCmd)
	getModifiedFilesCmd.Flags().StringVar(&getModifiedFilesArgs.RepoURL, "repo-url", "", "Git repository URL")
	getModifiedFilesCmd.Flags().StringVar(&getModifiedFilesArgs.CommitHash1, "commit-hash1", "", "First commit hash")
	getModifiedFilesCmd.Flags().StringVar(&getModifiedFilesArgs.CommitHash2, "commit-hash2", "", "Second commit hash")
	_ = getModifiedFilesCmd.MarkFlagRequired("repo-url")
	_ = getModifiedFilesCmd.MarkFlagRequired("commit-hash1")
	_ = getModifiedFilesCmd.MarkFlagRequired("commit-hash2")

	getModifiedFilesCmd.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	getModifiedFilesCmd.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	getModifiedFilesCmd.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")

	getModifiedFilesCmd.Flags().StringVar(&getModifiedFilesOutputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
}
