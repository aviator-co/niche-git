// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"

	"github.com/aviator-co/niche-git/debug"
	"github.com/aviator-co/niche-git/internal/diff"
	"github.com/aviator-co/niche-git/internal/fetch"
	"github.com/bmatcuk/doublestar"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/storage/memory"
)

type ModifiedFilePattern struct {
	// FilePathPattern is a list of doublestar matching patterns for the file paths.
	FilePathPattern []string
	// FileContentPattern is an optional regular expression pattern for the file content.
	FileContentPattern *regexp.Regexp
}

type ModificationStatus string

const (
	ModificationStatusAdded    ModificationStatus = "ADDED"
	ModificationStatusDeleted  ModificationStatus = "DELETED"
	ModificationStatusModified ModificationStatus = "MODIFIED"
)

type ModifiedFilePatternMatch struct {
	Before int `json:"before"`
	After  int `json:"after"`
}

type ModifiedFile struct {
	Path    string                               `json:"path"`
	Status  ModificationStatus                   `json:"status"`
	Matches map[string]*ModifiedFilePatternMatch `json:"matches,omitempty"`
}

func FetchModifiedFilesWithRegexpMatch(
	ctx context.Context,
	repoURL string,
	client *http.Client,
	commitHash1, commitHash2 plumbing.Hash,
	patterns map[string]ModifiedFilePattern,
) ([]*ModifiedFile, debug.FetchDebugInfo, *debug.FetchDebugInfo, error) {
	packfilebs, fetchDebugInfo, err := fetch.FetchBlobNonePackfile(ctx, repoURL, client, []plumbing.Hash{commitHash1, commitHash2}, 1)
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

	tree1, err := getTreeFromCommit(storage, commitHash1)
	if err != nil {
		return nil, fetchDebugInfo, nil, err
	}
	tree2, err := getTreeFromCommit(storage, commitHash2)
	if err != nil {
		return nil, fetchDebugInfo, nil, err
	}

	modified, err := diff.DiffTree(storage, tree1, tree2)
	if err != nil {
		return nil, fetchDebugInfo, nil, fmt.Errorf("failed to take file diffs: %v", err)
	}

	packfilebs, fetchBlobDebugInfo, err := fetch.FetchBlobPackfile(ctx, repoURL, client, getBlobHashes(modified))
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

	var ret []*ModifiedFile
	for path, hashes := range modified {
		m, err := matchWithPattern(storage, patterns, path, hashes)
		if err != nil {
			return nil, fetchDebugInfo, blobFetchDebugInfo, err
		}
		ret = append(ret, m)
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Path < ret[j].Path
	})
	return ret, fetchDebugInfo, blobFetchDebugInfo, nil
}

func getBlobHashes(modified map[string]diff.BlobHashes) []plumbing.Hash {
	m := map[plumbing.Hash]struct{}{}
	for _, hashes := range modified {
		if !hashes.BlobHash1.IsZero() {
			m[hashes.BlobHash1] = struct{}{}
		}
		if !hashes.BlobHash2.IsZero() {
			m[hashes.BlobHash2] = struct{}{}
		}
	}
	hashes := make([]plumbing.Hash, 0, len(m))
	for hash := range m {
		hashes = append(hashes, hash)
	}
	return hashes
}

func matchWithPattern(storage *memory.Storage, patterns map[string]ModifiedFilePattern, path string, blobs diff.BlobHashes) (*ModifiedFile, error) {
	status := ModificationStatusModified
	var content1 []byte
	var content2 []byte
	if blobs.BlobHash1.IsZero() {
		// File is added.
		status = ModificationStatusAdded
	} else {
		var err error
		content1, err = getBlobContent(storage, blobs.BlobHash1)
		if err != nil {
			return nil, fmt.Errorf("cannot read the file %q: %v", path, err)
		}
	}
	if blobs.BlobHash2.IsZero() {
		// File is deleted.
		status = ModificationStatusDeleted
	} else {
		var err error
		content2, err = getBlobContent(storage, blobs.BlobHash2)
		if err != nil {
			return nil, fmt.Errorf("cannot read the file %q: %v", path, err)
		}
	}
	matches := map[string]*ModifiedFilePatternMatch{}
	for name, pattern := range patterns {
		match, err := matchPattern(pattern, path, status, content1, content2)
		if err != nil {
			return nil, fmt.Errorf("cannot match the pattern %q: %v", name, err)
		}
		if match != nil {
			matches[name] = match
		}
	}
	ret := &ModifiedFile{
		Path:   path,
		Status: status,
	}
	if len(matches) > 0 {
		ret.Matches = matches
	}
	return ret, nil
}

func getBlobContent(storage *memory.Storage, hash plumbing.Hash) ([]byte, error) {
	blob, err := storage.EncodedObject(plumbing.BlobObject, hash)
	if err != nil {
		return nil, fmt.Errorf("cannot open the hash %q: %v", hash.String(), err)
	}
	rd, err := blob.Reader()
	if err != nil {
		return nil, fmt.Errorf("cannot open the blob: %v", err)
	}
	defer rd.Close()
	bs, err := io.ReadAll(rd)
	if err != nil {
		return nil, fmt.Errorf("cannot read the blob: %v", err)
	}
	return bs, nil
}

func matchPattern(pattern ModifiedFilePattern, path string, status ModificationStatus, content1, content2 []byte) (*ModifiedFilePatternMatch, error) {
	if matched, err := matchPath(pattern.FilePathPattern, path); err != nil {
		return nil, err
	} else if !matched {
		return nil, nil
	}
	if pattern.FileContentPattern == nil {
		switch status {
		case ModificationStatusAdded:
			return &ModifiedFilePatternMatch{Before: 0, After: 1}, nil
		case ModificationStatusDeleted:
			return &ModifiedFilePatternMatch{Before: 1, After: 0}, nil
		case ModificationStatusModified:
			return &ModifiedFilePatternMatch{Before: 1, After: 1}, nil
		}
	}
	before := len(pattern.FileContentPattern.FindAll(content1, -1))
	after := len(pattern.FileContentPattern.FindAll(content2, -1))
	if before == 0 && after == 0 {
		// No match.
		return nil, nil
	}
	return &ModifiedFilePatternMatch{Before: before, After: after}, nil
}

func matchPath(doublestartPatterns []string, path string) (bool, error) {
	for i := len(doublestartPatterns) - 1; i >= 0; i-- {
		pat := doublestartPatterns[i]
		resultIfMatched := true
		if pat[0] == '!' {
			resultIfMatched = false
			pat = pat[1:]
		}
		if matched, err := doublestar.Match(pat, path); err != nil {
			return false, err
		} else if matched {
			return resultIfMatched, nil
		}
	}
	return false, nil
}
