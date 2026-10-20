// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"path"
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
	// Symlink entries hold their target path as blob content, so they need a
	// round-trip before they can be resolved. Collect them separately and
	// fetch them alongside the regular blobs below.
	symlinkBlobs := make(map[string]plumbing.Hash)
	for _, filePath := range filePaths {
		blobHash, mode, err := resolveEntryFromTree(storage, tree, filePath)
		if err != nil {
			return nil, fetchDebugInfo, nil, fmt.Errorf("failed to get a blob hash for %s: %v", filePath, err)
		}
		if blobHash == plumbing.ZeroHash {
			continue
		}
		if mode == filemode.Symlink {
			symlinkBlobs[filePath] = blobHash
		} else {
			blobs[filePath] = blobHash
		}
	}

	if len(blobs) == 0 && len(symlinkBlobs) == 0 {
		return make(map[string]string), fetchDebugInfo, nil, nil
	}

	fetched := make(map[plumbing.Hash]bool)
	var blobHashes []plumbing.Hash
	for _, blobHash := range blobs {
		if !fetched[blobHash] {
			fetched[blobHash] = true
			blobHashes = append(blobHashes, blobHash)
		}
	}
	for _, blobHash := range symlinkBlobs {
		if !fetched[blobHash] {
			fetched[blobHash] = true
			blobHashes = append(blobHashes, blobHash)
		}
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

	if len(symlinkBlobs) > 0 {
		if err := resolveSymlinks(ctx, repoURL, client, storage, tree, symlinkBlobs, blobs, fetched); err != nil {
			return nil, fetchDebugInfo, blobFetchDebugInfo, err
		}
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

// resolveEntryFromTree returns the blob hash and file mode for filePath. A
// symlink resolves to its own blob, whose content is the target path; turning
// that into the target's content is the caller's job, since it needs the blob
// fetched first.
func resolveEntryFromTree(storage *memory.Storage, tree *object.Tree, filePath string) (plumbing.Hash, filemode.FileMode, error) {
	first, second, _ := strings.Cut(filePath, "/")
	for _, entry := range tree.Entries {
		if entry.Name != first {
			continue
		}
		if entry.Mode == filemode.Dir {
			subTree, err := object.GetTree(storage, entry.Hash)
			if err != nil {
				return plumbing.ZeroHash, filemode.Empty, err
			}
			return resolveEntryFromTree(storage, subTree, second)
		}
		if second == "" {
			switch entry.Mode {
			case filemode.Regular, filemode.Executable, filemode.Symlink:
				return entry.Hash, entry.Mode, nil
			}
		}
		// A non-directory in the middle of the path, or an entry we cannot
		// read (submodule): treat as not found.
		return plumbing.ZeroHash, filemode.Empty, nil
	}
	// The file does not exist.
	return plumbing.ZeroHash, filemode.Empty, nil
}

// resolveSymlinks reads each symlink blob, resolves its target against the
// tree, and records the target's blob under the ORIGINAL requested path so
// callers get the content they asked for. Targets not already fetched are
// pulled in one extra packfile request, which only happens when the requested
// paths actually contain symlinks.
//
// Only one hop is followed: a symlink pointing at another symlink reads as
// not found, which keeps this free of cycle detection.
func resolveSymlinks(
	ctx context.Context,
	repoURL string,
	client *http.Client,
	storage *memory.Storage,
	tree *object.Tree,
	symlinkBlobs map[string]plumbing.Hash,
	blobs map[string]plumbing.Hash,
	fetched map[plumbing.Hash]bool,
) error {
	targets := make(map[string]plumbing.Hash)
	var toFetch []plumbing.Hash
	for filePath, linkHash := range symlinkBlobs {
		bs, err := getBlobContent(storage, linkHash)
		if err != nil {
			return fmt.Errorf("failed to read the symlink %s: %v", filePath, err)
		}
		targetPath := symlinkTargetPath(filePath, string(bs))
		if targetPath == "" {
			continue
		}
		targetHash, mode, err := resolveEntryFromTree(storage, tree, targetPath)
		if err != nil {
			return fmt.Errorf("failed to resolve the symlink %s: %v", filePath, err)
		}
		if targetHash == plumbing.ZeroHash || mode == filemode.Symlink {
			continue
		}
		targets[filePath] = targetHash
		if !fetched[targetHash] {
			fetched[targetHash] = true
			toFetch = append(toFetch, targetHash)
		}
	}

	if len(toFetch) > 0 {
		packfilebs, _, err := fetch.FetchBlobPackfile(ctx, repoURL, client, toFetch)
		if err != nil {
			return err
		}
		parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), storage)
		if err != nil {
			return fmt.Errorf("failed to parse packfile: %v", err)
		}
		if _, err := parser.Parse(); err != nil {
			return fmt.Errorf("failed to parse packfile: %v", err)
		}
	}

	for filePath, targetHash := range targets {
		blobs[filePath] = targetHash
	}
	return nil
}

// symlinkTargetPath resolves a symlink's target against the directory holding
// the link. Absolute targets and targets escaping the repository root have no
// in-tree blob to read, so they resolve to the empty string.
func symlinkTargetPath(linkPath, target string) string {
	target = strings.TrimSpace(target)
	if target == "" || strings.HasPrefix(target, "/") {
		return ""
	}
	resolved := path.Clean(path.Join(path.Dir(linkPath), target))
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return ""
	}
	return resolved
}
