// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"
	"sort"

	nichegit "github.com/aviator-co/niche-git"
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
		files, debugInfo, fetchErr := nichegit.FetchModifiedFiles(getModifiedFilesArgs.repoURL, client, getModifiedFilesArgs.commitHash1, getModifiedFilesArgs.commitHash2)
		if files == nil {
			// Always create an empty slice for JSON output.
			files = []string{}
		}
		output := getModifiedFilesOutput{
			Files:           files,
			ResponseHeaders: debugInfo.ResponseHeaders,
			PackfileSize:    debugInfo.PackfileSize,
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
	Files           []string            `json:"files"`
	ResponseHeaders map[string][]string `json:"responseHeaders"`
	PackfileSize    int                 `json:"packfileSize"`
	Error           string              `json:"error,omitempty"`
}

func init() {
	rootCmd.AddCommand(getModifiedFilesCmd)
	getModifiedFilesCmd.Flags().StringVar(&getModifiedFilesArgs.repoURL, "repo-url", "", "Git reposiotry URL")
	getModifiedFilesCmd.Flags().StringVar(&getModifiedFilesArgs.commitHash1, "commit-hash1", "", "First commit hash")
	getModifiedFilesCmd.Flags().StringVar(&getModifiedFilesArgs.commitHash2, "commit-hash2", "", "Second commit hash")
	getModifiedFilesCmd.MarkFlagRequired("repo-url")
	getModifiedFilesCmd.MarkFlagRequired("commit-hash1")
	getModifiedFilesCmd.MarkFlagRequired("commit-hash2")

	getModifiedFilesCmd.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	getModifiedFilesCmd.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	getModifiedFilesCmd.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")

	getModifiedFilesCmd.Flags().StringVar(&getModifiedFilesArgs.outputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
}
