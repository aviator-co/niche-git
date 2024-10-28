// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/aviator-co/niche-git/debug"
	"github.com/go-git/go-git/v5/plumbing"
)

type GetFilesArgs struct {
	RepoURL    string   `json:"repoURL"`
	CommitHash string   `json:"commitHash"`
	FilePaths  []string `json:"filePaths"`
}

type getFilesOutput struct {
	Files              map[string]string     `json:"files"`
	FetchDebugInfo     *debug.FetchDebugInfo `json:"fetchDebugInfo"`
	BlobFetchDebugInfo *debug.FetchDebugInfo `json:"blobFetchDebugInfo"`
	Error              string                `json:"error,omitempty"`
}

func GetFiles(args GetFilesArgs) *getFilesOutput {
	client := &http.Client{Transport: &authnRoundtripper{}}
	files, fetchDebugInfo, blobFetchDebugInfo, err := nichegit.FetchFiles(
		args.RepoURL,
		client,
		plumbing.NewHash(args.CommitHash),
		args.FilePaths,
	)
	if files == nil {
		files = make(map[string]string)
	}
	output := &getFilesOutput{
		Files:              files,
		FetchDebugInfo:     &fetchDebugInfo,
		BlobFetchDebugInfo: blobFetchDebugInfo,
	}
	if err != nil {
		output.Error = err.Error()
	}
	return output
}
