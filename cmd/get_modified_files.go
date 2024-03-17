// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/spf13/cobra"
)

var (
	repoURL     string
	commitHash1 string
	commitHash2 string

	authzHeader        string
	basicAuthzUser     string
	basicAuthzPassword string

	outputFile string
)

var getModifiedFilesCmd = &cobra.Command{
	Use: "get-modified-files",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := &http.Client{Transport: &authnRoundtripper{}}
		files, debugInfo, fetchErr := nichegit.FetchModifiedFiles(repoURL, client, commitHash1, commitHash2)
		output := getModifiedFilesOutput{
			Files:           files,
			ResponseHeaders: debugInfo.ResponseHeaders,
			PackfileSize:    debugInfo.PackfileSize,
		}
		sort.Strings(output.Files)
		if fetchErr != nil {
			output.Error = fetchErr.Error()
		}
		var of io.Writer
		if outputFile == "-" {
			of = os.Stdout
		} else {
			file, err := os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			defer file.Close()
			of = file
		}
		enc := json.NewEncoder(of)
		enc.SetIndent("", "  ")
		if err := enc.Encode(output); err != nil {
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
	getModifiedFilesCmd.Flags().StringVar(&repoURL, "repo-url", "", "Git reposiotry URL")
	getModifiedFilesCmd.Flags().StringVar(&commitHash1, "commit-hash1", "", "First commit hash")
	getModifiedFilesCmd.Flags().StringVar(&commitHash2, "commit-hash2", "", "Second commit hash")
	getModifiedFilesCmd.MarkFlagRequired("repo-url")
	getModifiedFilesCmd.MarkFlagRequired("commit-hash1")
	getModifiedFilesCmd.MarkFlagRequired("commit-hash2")

	getModifiedFilesCmd.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	getModifiedFilesCmd.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	getModifiedFilesCmd.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")

	getModifiedFilesCmd.Flags().StringVar(&outputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
}

type authnRoundtripper struct{}

func (rt *authnRoundtripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if authzHeader != "" {
		req.Header.Set("Authorization", authzHeader)
	} else if basicAuthzUser != "" && basicAuthzPassword != "" {
		req.SetBasicAuth(basicAuthzUser, basicAuthzPassword)
	}
	return http.DefaultTransport.RoundTrip(req)
}
