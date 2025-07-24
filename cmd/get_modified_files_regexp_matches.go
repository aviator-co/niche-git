// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"net/http"
	"regexp"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/aviator-co/niche-git/debug"
	"github.com/go-git/go-git/v5/plumbing"
)

type GetModifiedFilesPattern struct {
	FilePathPatterns   []string `json:"filePathPatterns"`
	FileContentPattern string   `json:"fileContentPattern,omitempty"`
}

type GetModifiedFilesRegexpMatchesArgs struct {
	RepoURL     string                             `json:"repoURL"`
	CommitHash1 string                             `json:"commitHash1"`
	CommitHash2 string                             `json:"commitHash2"`
	Patterns    map[string]GetModifiedFilesPattern `json:"patterns"`
}

type getModifiedFilesRegexpMatchesOutput struct {
	Files              []*nichegit.ModifiedFile `json:"files"`
	FetchDebugInfo     *debug.FetchDebugInfo    `json:"fetchDebugInfo"`
	BlobFetchDebugInfo *debug.FetchDebugInfo    `json:"blobFetchDebugInfo"`
	Error              string                   `json:"error,omitempty"`
}

func GetModifiedFilesRegexpMatches(ctx context.Context, args GetModifiedFilesRegexpMatchesArgs) *getModifiedFilesRegexpMatchesOutput {
	patterns := make(map[string]nichegit.ModifiedFilePattern)
	for key, value := range args.Patterns {
		v := nichegit.ModifiedFilePattern{
			FilePathPattern: value.FilePathPatterns,
		}
		if value.FileContentPattern != "" {
			pattern, err := regexp.Compile(value.FileContentPattern)
			if err != nil {
				return &getModifiedFilesRegexpMatchesOutput{Error: err.Error()}
			}
			v.FileContentPattern = pattern
		}
		patterns[key] = v
	}

	client := &http.Client{Transport: &authnRoundtripper{}}
	files, fetchDebugInfo, blobFetchDebugInfo, err := nichegit.FetchModifiedFilesWithRegexpMatch(
		ctx,
		args.RepoURL,
		client,
		plumbing.NewHash(args.CommitHash1),
		plumbing.NewHash(args.CommitHash2),
		patterns,
	)
	output := &getModifiedFilesRegexpMatchesOutput{
		Files:              files,
		FetchDebugInfo:     &fetchDebugInfo,
		BlobFetchDebugInfo: blobFetchDebugInfo,
	}
	if err != nil {
		output.Error = err.Error()
	}
	return output
}
