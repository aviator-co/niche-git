// Copyright 2026 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/spf13/cobra"
)

var (
	mergeTreeArgs       nichegit.MergeTreeArgs
	mergeTreeOutputFile string
)

var mergeTree = &cobra.Command{
	Use: "merge-tree",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client := &http.Client{Transport: &authnRoundtripper{}}
		output := nichegit.MergeTree(ctx, client, mergeTreeArgs)
		if err := writeJSON(mergeTreeOutputFile, output); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mergeTree)
	mergeTree.Flags().StringVar(&mergeTreeArgs.RepoURL, "repo-url", "", "Git repository URL")
	mergeTree.Flags().StringVar(&mergeTreeArgs.Commit1, "commit1", "", "First commit hash to merge")
	mergeTree.Flags().StringVar(&mergeTreeArgs.Commit2, "commit2", "", "Second commit hash to merge")
	mergeTree.Flags().StringVar(&mergeTreeArgs.MergeBase, "merge-base", "", "Merge base commit hash of commit1 and commit2")
	_ = mergeTree.MarkFlagRequired("repo-url")
	_ = mergeTree.MarkFlagRequired("commit1")
	_ = mergeTree.MarkFlagRequired("commit2")
	_ = mergeTree.MarkFlagRequired("merge-base")

	mergeTree.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	mergeTree.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	mergeTree.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")

	mergeTree.Flags().StringVar(&mergeTreeOutputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
}
