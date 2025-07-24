// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/aviator-co/niche-git/debug"
	"github.com/aviator-co/niche-git/internal/fetch"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

type GetFilesArgs struct {
	RepoURL    string   `json:"repoURL"`
	CommitHash string   `json:"commitHash"`
	FilePaths  []string `json:"filePaths"`
}

type GetFilesOutput struct {
	Files              map[string]string     `json:"files"`
	FetchDebugInfo     *debug.FetchDebugInfo `json:"fetchDebugInfo"`
	BlobFetchDebugInfo *debug.FetchDebugInfo `json:"blobFetchDebugInfo"`
	Error              string                `json:"error,omitempty"`
}

func GetFiles(ctx context.Context, client *http.Client, args GetFilesArgs) GetFilesOutput {
	files, fetchDebugInfo, blobFetchDebugInfo, err := fetchFiles(
		ctx,
		args.RepoURL,
		client,
		plumbing.NewHash(args.CommitHash),
		args.FilePaths,
	)
	if files == nil {
		files = make(map[string]string)
	}
	output := GetFilesOutput{
		Files:              files,
		FetchDebugInfo:     &fetchDebugInfo,
		BlobFetchDebugInfo: blobFetchDebugInfo,
	}
	if err != nil {
		output.Error = err.Error()
	}
	return output
}

func fetchFiles(ctx context.Context, repoURL string, client *http.Client, commitHash plumbing.Hash, filePaths []string) (map[string]string, debug.FetchDebugInfo, *debug.FetchDebugInfo, error) {
	packfilebs, fetchDebugInfo, err := fetch.FetchBlobNonePackfile(ctx, repoURL, client, []plumbing.Hash{commitHash}, 1)
	if err != nil {
		return nil, fetchDebugInfo, nil, err
	}
	storage := memory.NewStorage()
	parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), storage)
	if err != nil {
		return nil, fetchDebugInfo, nil, fmt.Errorf("failed to parse packfile: %v", err)
	}
	if _, err := parser.Parse(); err != nil {
		return nil, fetchDebugInfo, nil, fmt.Errorf("failed to parse packfile: %v", err)
	}

	tree, err := getTreeFromCommit(storage, commitHash)
	if err != nil {
		return nil, fetchDebugInfo, nil, err
	}
	blobs := make(map[string]plumbing.Hash)
	for _, filePath := range filePaths {
		blobHash, err := getBlobHashFromTree(storage, tree, filePath)
		if err != nil {
			return nil, fetchDebugInfo, nil, fmt.Errorf("failed to get a blob hash for %s: %v", filePath, err)
		}
		if blobHash != plumbing.ZeroHash {
			blobs[filePath] = blobHash
		}
	}

	if len(blobs) == 0 {
		return make(map[string]string), fetchDebugInfo, nil, nil
	}

	var blobHashes []plumbing.Hash
	for _, blobHash := range blobs {
		blobHashes = append(blobHashes, blobHash)
	}

	packfilebs, fetchBlobDebugInfo, err := fetch.FetchBlobPackfile(ctx, repoURL, client, blobHashes)
	blobFetchDebugInfo := &fetchBlobDebugInfo
	if err != nil {
		return nil, fetchDebugInfo, blobFetchDebugInfo, err
	}
	parser, err = packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), storage)
	if err != nil {
		return nil, fetchDebugInfo, blobFetchDebugInfo, fmt.Errorf("failed to parse packfile: %v", err)
	}
	if _, err := parser.Parse(); err != nil {
		return nil, fetchDebugInfo, blobFetchDebugInfo, fmt.Errorf("failed to parse packfile: %v", err)
	}

	files := make(map[string]string)
	for filePath, blobHash := range blobs {
		bs, err := getBlobContent(storage, blobHash)
		if err != nil {
			return nil, fetchDebugInfo, blobFetchDebugInfo, fmt.Errorf("failed to get a blob content for %s: %v", filePath, err)
		}
		files[filePath] = string(bs)
	}
	return files, fetchDebugInfo, blobFetchDebugInfo, nil
}

func getBlobHashFromTree(storage *memory.Storage, tree *object.Tree, filePath string) (plumbing.Hash, error) {
	first, second, _ := strings.Cut(filePath, "/")
	for _, entry := range tree.Entries {
		if entry.Name == first {
			if entry.Mode == filemode.Regular || entry.Mode == filemode.Executable {
				if second == "" {
					return entry.Hash, nil
				}
				// Treat this as not found.
				return plumbing.ZeroHash, nil
			}
			if entry.Mode == filemode.Dir {
				subTree, err := object.GetTree(storage, entry.Hash)
				if err != nil {
					return plumbing.ZeroHash, err
				}
				return getBlobHashFromTree(storage, subTree, second)
			}
			// Treat this as not found.
			return plumbing.ZeroHash, nil
		}
	}
	// The file does not exist.
	return plumbing.ZeroHash, nil
}
