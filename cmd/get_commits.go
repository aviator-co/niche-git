// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/spf13/cobra"
)

var (
	getCommitsArgs struct {
		repoURL          string
		wantCommitHashes []string
		haveCommitHashes []string

		outputFile string
	}
)

var getCommitsCmd = &cobra.Command{
	Use: "get-commits",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := &http.Client{Transport: &authnRoundtripper{}}
		commits, debugInfo, fetchErr := nichegit.FetchCommits(getCommitsArgs.repoURL, client, getCommitsArgs.wantCommitHashes, getCommitsArgs.haveCommitHashes)
		if commits == nil {
			// Always create an empty slice for JSON output.
			commits = []*nichegit.CommitInfo{}
		}
		output := getCommitsOutput{
			Commits:         commits,
			ResponseHeaders: debugInfo.ResponseHeaders,
			PackfileSize:    debugInfo.PackfileSize,
		}
		if fetchErr != nil {
			output.Error = fetchErr.Error()
		}
		if err := writeJSON(getCommitsArgs.outputFile, output); err != nil {
			return err
		}
		return fetchErr
	},
}

type getCommitsOutput struct {
	Commits         []*nichegit.CommitInfo `json:"commits"`
	ResponseHeaders map[string][]string    `json:"responseHeaders"`
	PackfileSize    int                    `json:"packfileSize"`
	Error           string                 `json:"error,omitempty"`
}

func init() {
	rootCmd.AddCommand(getCommitsCmd)
	getCommitsCmd.Flags().StringVar(&getCommitsArgs.repoURL, "repo-url", "", "Git reposiotry URL")
	getCommitsCmd.Flags().StringSliceVar(&getCommitsArgs.wantCommitHashes, "want-commit-hashes", nil, "Want commit hashes")
	getCommitsCmd.Flags().StringSliceVar(&getCommitsArgs.haveCommitHashes, "have-commit-hashes", nil, "Have commit hashes")
	getCommitsCmd.MarkFlagRequired("repo-url")

	getCommitsCmd.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	getCommitsCmd.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	getCommitsCmd.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")

	getCommitsCmd.Flags().StringVar(&getCommitsArgs.outputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
}
