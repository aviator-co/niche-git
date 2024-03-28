// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/spf13/cobra"
)

var (
	lsRefsArgs struct {
		repoURL     string
		refPrefixes []string

		outputFile string
	}
)

var lsRefsCmd = &cobra.Command{
	Use: "ls-refs",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := &http.Client{Transport: &authnRoundtripper{}}
		refs, debugInfo, fetchErr := nichegit.LsRefs(lsRefsArgs.repoURL, client, lsRefsArgs.refPrefixes)
		if refs == nil {
			// Always create an empty slice for JSON output.
			refs = []*nichegit.RefInfo{}
		}
		output := lsRefsOutput{
			Refs:            refs,
			ResponseHeaders: debugInfo.ResponseHeaders,
		}
		if fetchErr != nil {
			output.Error = fetchErr.Error()
		}
		if err := writeJSON(lsRefsArgs.outputFile, output); err != nil {
			return err
		}
		return fetchErr
	},
}

type lsRefsOutput struct {
	Refs            []*nichegit.RefInfo `json:"refs"`
	ResponseHeaders map[string][]string `json:"responseHeaders"`
	Error           string              `json:"error,omitempty"`
}

func init() {
	rootCmd.AddCommand(lsRefsCmd)
	lsRefsCmd.Flags().StringVar(&lsRefsArgs.repoURL, "repo-url", "", "Git reposiotry URL")
	lsRefsCmd.Flags().StringSliceVar(&lsRefsArgs.refPrefixes, "ref-prefixes", nil, "Ref prefixes")
	lsRefsCmd.MarkFlagRequired("repo-url")

	lsRefsCmd.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	lsRefsCmd.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	lsRefsCmd.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")

	lsRefsCmd.Flags().StringVar(&lsRefsArgs.outputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
}
