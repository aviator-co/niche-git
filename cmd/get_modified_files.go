// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"
	"sort"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/aviator-co/niche-git/debug"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

var (
	getModifiedFilesArgs struct {
		repoURL     string
		commitHash1 string
		commitHash2 string

		outputFile string
	}
)

var getModifiedFilesCmd = &cobra.Command{
	Use: "get-modified-files",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := &http.Client{Transport: &authnRoundtripper{}}
		files, debugInfo, fetchErr := nichegit.FetchModifiedFiles(
			getModifiedFilesArgs.repoURL,
			client,
			plumbing.NewHash(getModifiedFilesArgs.commitHash1),
			plumbing.NewHash(getModifiedFilesArgs.commitHash2),
		)
		if files == nil {
			// Always create an empty slice for JSON output.
			files = []string{}
		}
		output := getModifiedFilesOutput{
			Files:     files,
			DebugInfo: debugInfo,
		}
		sort.Strings(output.Files)
		if fetchErr != nil {
			output.Error = fetchErr.Error()
		}
		if err := writeJSON(getModifiedFilesArgs.outputFile, output); err != nil {
			return err
		}
		return fetchErr
	},
}

type getModifiedFilesOutput struct {
	Files     []string             `json:"files"`
	DebugInfo debug.FetchDebugInfo `json:"debugInfo"`
	Error     string               `json:"error,omitempty"`
}

func init() {
	rootCmd.AddCommand(getModifiedFilesCmd)
	getModifiedFilesCmd.Flags().StringVar(&getModifiedFilesArgs.repoURL, "repo-url", "", "Git repository URL")
	getModifiedFilesCmd.Flags().StringVar(&getModifiedFilesArgs.commitHash1, "commit-hash1", "", "First commit hash")
	getModifiedFilesCmd.Flags().StringVar(&getModifiedFilesArgs.commitHash2, "commit-hash2", "", "Second commit hash")
	_ = getModifiedFilesCmd.MarkFlagRequired("repo-url")
	_ = getModifiedFilesCmd.MarkFlagRequired("commit-hash1")
	_ = getModifiedFilesCmd.MarkFlagRequired("commit-hash2")

	getModifiedFilesCmd.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	getModifiedFilesCmd.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	getModifiedFilesCmd.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")

	getModifiedFilesCmd.Flags().StringVar(&getModifiedFilesArgs.outputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
}
