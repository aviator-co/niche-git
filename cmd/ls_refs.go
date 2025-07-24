// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/spf13/cobra"
)

var (
	lsRefsArgs       nichegit.LsRefsArgs
	lsRefsOutputFile string
)

var lsRefsCmd = &cobra.Command{
	Use: "ls-refs",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client := &http.Client{Transport: &authnRoundtripper{}}
		output := nichegit.LsRefs(ctx, client, lsRefsArgs)
		if err := writeJSON(lsRefsOutputFile, output); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(lsRefsCmd)
	lsRefsCmd.Flags().StringVar(&lsRefsArgs.RepoURL, "repo-url", "", "Git repository URL")
	lsRefsCmd.Flags().StringSliceVar(&lsRefsArgs.RefPrefixes, "ref-prefixes", nil, "Ref prefixes")
	_ = lsRefsCmd.MarkFlagRequired("repo-url")

	lsRefsCmd.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	lsRefsCmd.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	lsRefsCmd.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")

	lsRefsCmd.Flags().StringVar(&lsRefsOutputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
}
