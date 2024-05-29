// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/aviator-co/niche-git/debug"
	"github.com/go-git/go-git/v5/plumbing"
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
		var wantCommitHashes []plumbing.Hash
		for _, s := range getCommitsArgs.wantCommitHashes {
			wantCommitHashes = append(wantCommitHashes, plumbing.NewHash(s))
		}
		var haveCommitHashes []plumbing.Hash
		for _, s := range getCommitsArgs.haveCommitHashes {
			haveCommitHashes = append(haveCommitHashes, plumbing.NewHash(s))
		}
		client := &http.Client{Transport: &authnRoundtripper{}}
		commits, debugInfo, fetchErr := nichegit.FetchCommits(getCommitsArgs.repoURL, client, wantCommitHashes, haveCommitHashes)
		if commits == nil {
			// Always create an empty slice for JSON output.
			commits = []*nichegit.CommitInfo{}
		}
		output := getCommitsOutput{
			Commits:   commits,
			DebugInfo: debugInfo,
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
	Commits   []*nichegit.CommitInfo `json:"commits"`
	DebugInfo debug.FetchDebugInfo   `json:"debugInfo"`
	Error     string                 `json:"error,omitempty"`
}

func init() {
	rootCmd.AddCommand(getCommitsCmd)
	getCommitsCmd.Flags().StringVar(&getCommitsArgs.repoURL, "repo-url", "", "Git reposiotry URL")
	getCommitsCmd.Flags().StringSliceVar(&getCommitsArgs.wantCommitHashes, "want-commit-hashes", nil, "Want commit hashes")
	getCommitsCmd.Flags().StringSliceVar(&getCommitsArgs.haveCommitHashes, "have-commit-hashes", nil, "Have commit hashes")
	_ = getCommitsCmd.MarkFlagRequired("repo-url")

	getCommitsCmd.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	getCommitsCmd.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	getCommitsCmd.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")

	getCommitsCmd.Flags().StringVar(&getCommitsArgs.outputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
}
