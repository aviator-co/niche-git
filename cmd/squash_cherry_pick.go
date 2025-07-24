// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/spf13/cobra"
)

var (
	squashCherryPickArgs       nichegit.SquashCherryPickArgs
	squashCherryPickOutputFile string
)

var squashCherryPick = &cobra.Command{
	Use: "squash-cherry-pick",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client := &http.Client{Transport: &authnRoundtripper{}}
		output := nichegit.SquashCherryPick(ctx, client, squashCherryPickArgs)
		if err := writeJSON(squashCherryPickOutputFile, output); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(squashCherryPick)
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.RepoURL, "repo-url", "", "Git repository URL")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.CherryPickFrom, "cherry-pick-from", "", "Commit hash where cherry-pick from")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.CherryPickTo, "cherry-pick-to", "", "Commit hash where cherry-pick to")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.CherryPickBase, "cherry-pick-base", "", "The merge base of the cherry-pick from. The changes from this commit to cherry-pick-from will be applied to cherry-pick-to.")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.CommitMessage, "commit-message", "", "Commit message of the squashed commit")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.Author, "author", "", "Author name")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.AuthorEmail, "author-email", "", "Author email address")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.AuthorTime, "author-time", "", "Author time in RFC3339 format (e.g. 2024-01-01T00:00:00Z)")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.Committer, "committer", "", "Committer name")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.CommitterEmail, "committer-email", "", "Committer email address")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.CommitterTime, "committer-time", "", "Commit time in RFC3339 format (e.g. 2024-01-01T00:00:00Z)")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.Ref, "ref", "", "A ref name (e.g. refs/heads/foobar) to push")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.ConflictRef, "conflict-ref", "", "A ref name (e.g. refs/heads/foobar) to push when there's a conflict")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.CurrentRefHash, "current-ref-hash", "", "The expected current commit hash of the ref. If this is specified, the push will use the current commit hash. This is used for compare-and-swap.")
	squashCherryPick.Flags().BoolVar(&squashCherryPickArgs.AbortOnConflict, "abort-on-conflict", false, "Abort the operation if there is a merge conflict")
	_ = squashCherryPick.MarkFlagRequired("repo-url")
	_ = squashCherryPick.MarkFlagRequired("cherry-pick-from")
	_ = squashCherryPick.MarkFlagRequired("cherry-pick-to")
	_ = squashCherryPick.MarkFlagRequired("cherry-pick-base")
	_ = squashCherryPick.MarkFlagRequired("commit-message")
	_ = squashCherryPick.MarkFlagRequired("author")
	_ = squashCherryPick.MarkFlagRequired("author-email")
	_ = squashCherryPick.MarkFlagRequired("committer")
	_ = squashCherryPick.MarkFlagRequired("committer-email")
	_ = squashCherryPick.MarkFlagRequired("ref")

	squashCherryPick.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	squashCherryPick.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	squashCherryPick.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")

	squashCherryPick.Flags().StringVar(&squashCherryPickOutputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
}
