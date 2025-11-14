// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/aviator-co/niche-git/debug"
	"github.com/aviator-co/niche-git/internal/fetch"
	"github.com/aviator-co/niche-git/internal/merge"
	"github.com/aviator-co/niche-git/internal/push"
	"github.com/aviator-co/niche-git/internal/resolvediff3"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

type PushSquashCherryPickResult struct {
	CommitHash            plumbing.Hash
	CherryPickedFiles     []string
	ConflictOpenFiles     []string
	ConflictResolvedFiles []string
	BinaryConflictFiles   []string
	NonFileConflictFiles  []string
}

type SquashCherryPickArgs struct {
	RepoURL         string `json:"repoURL"`
	CherryPickFrom  string `json:"cherryPickFrom"`
	CherryPickTo    string `json:"cherryPickTo"`
	CherryPickBase  string `json:"cherryPickBase"`
	CommitMessage   string `json:"commitMessage"`
	Author          string `json:"author"`
	AuthorEmail     string `json:"authorEmail"`
	AuthorTime      string `json:"authorTime"`
	Committer       string `json:"committer"`
	CommitterEmail  string `json:"committerEmail"`
	CommitterTime   string `json:"committerTime"`
	Ref             string `json:"ref"`
	ConflictRef     string `json:"conflictRef"`
	CurrentRefHash  string `json:"currentRefHash"`
	AbortOnConflict bool   `json:"abortOnConflict"`
}

type SquashCherryPickOutput struct {
	CommitHash            string                `json:"commitHash"`
	CherryPickedFiles     []string              `json:"cherryPickedFiles"`
	ConflictOpenFiles     []string              `json:"conflictOpenFiles"`
	ConflictResolvedFiles []string              `json:"conflictResolvedFiles"`
	BinaryConflictFiles   []string              `json:"binaryConflictFiles"`
	NonFileConflictFiles  []string              `json:"nonFileConflictFiles"`
	FetchDebugInfo        debug.FetchDebugInfo  `json:"fetchDebugInfo"`
	BlobFetchDebugInfo    *debug.FetchDebugInfo `json:"blobFetchDebugInfo"`
	PushDebugInfo         *debug.PushDebugInfo  `json:"pushDebugInfo"`
	Error                 string                `json:"error,omitempty"`
}

func SquashCherryPick(ctx context.Context, client *http.Client, args SquashCherryPickArgs) SquashCherryPickOutput {
	var currentRefhash *plumbing.Hash
	if args.CurrentRefHash != "" {
		hash := plumbing.NewHash(args.CurrentRefHash)
		currentRefhash = &hash
	}
	author, err := newSignature(args.Author, args.AuthorEmail, args.AuthorTime)
	if err != nil {
		return SquashCherryPickOutput{Error: fmt.Sprintf("invalid author signature: %v", err)}
	}
	committer, err := newSignature(args.Committer, args.CommitterEmail, args.CommitterTime)
	if err != nil {
		return SquashCherryPickOutput{Error: fmt.Sprintf("invalid committer signature: %v", err)}
	}

	var conflictRef *plumbing.ReferenceName
	if args.ConflictRef != "" {
		r := plumbing.ReferenceName(args.ConflictRef)
		conflictRef = &r
	}
	result, fetchDebugInfo, blobFetchDebugInfo, pushDebugInfo, pushErr := pushSquashCherryPick(
		ctx,
		args.RepoURL,
		client,
		plumbing.NewHash(args.CherryPickFrom),
		plumbing.NewHash(args.CherryPickTo),
		plumbing.NewHash(args.CherryPickBase),
		args.CommitMessage,
		author,
		committer,
		plumbing.ReferenceName(args.Ref),
		conflictRef,
		currentRefhash,
		args.AbortOnConflict,
	)
	output := SquashCherryPickOutput{
		FetchDebugInfo:     fetchDebugInfo,
		BlobFetchDebugInfo: blobFetchDebugInfo,
		PushDebugInfo:      pushDebugInfo,
	}
	if result != nil {
		output.CommitHash = result.CommitHash.String()
		output.CherryPickedFiles = result.CherryPickedFiles
		output.ConflictOpenFiles = result.ConflictOpenFiles
		output.ConflictResolvedFiles = result.ConflictResolvedFiles
		output.BinaryConflictFiles = result.BinaryConflictFiles
		output.NonFileConflictFiles = result.NonFileConflictFiles
	}
	if output.CherryPickedFiles == nil {
		output.CherryPickedFiles = []string{}
	}
	if output.ConflictOpenFiles == nil {
		output.ConflictOpenFiles = []string{}
	}
	if output.ConflictResolvedFiles == nil {
		output.ConflictResolvedFiles = []string{}
	}
	if output.BinaryConflictFiles == nil {
		output.BinaryConflictFiles = []string{}
	}
	if output.NonFileConflictFiles == nil {
		output.NonFileConflictFiles = []string{}
	}
	if pushErr != nil {
		output.Error = pushErr.Error()
	}
	return output
}

func pushSquashCherryPick(
	ctx context.Context,
	repoURL string,
	client *http.Client,
	commitHashCherryPickFrom, commitHashCherryPickTo, commitHashCherryPickBase plumbing.Hash,
	commitMessage string,
	author, committer object.Signature,
	ref plumbing.ReferenceName,
	conflictRef *plumbing.ReferenceName,
	currentRefhash *plumbing.Hash,
	abortOnConflict bool,
) (*PushSquashCherryPickResult, debug.FetchDebugInfo, *debug.FetchDebugInfo, *debug.PushDebugInfo, error) {
	packfilebs, fetchDebugInfo, err := fetch.FetchBlobNonePackfile(ctx, repoURL, client, []plumbing.Hash{commitHashCherryPickFrom, commitHashCherryPickBase, commitHashCherryPickTo}, 1)
	if err != nil {
		return nil, fetchDebugInfo, nil, nil, err
	}

	storage := memory.NewStorage()
	parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), storage)
	if err != nil {
		return nil, fetchDebugInfo, nil, nil, fmt.Errorf("failed to parse packfile: %v", err)
	}
	if _, err := parser.Parse(); err != nil {
		return nil, fetchDebugInfo, nil, nil, fmt.Errorf("failed to parse packfile: %v", err)
	}

	treeCPFrom, err := getTreeFromCommit(storage, commitHashCherryPickFrom)
	if err != nil {
		return nil, fetchDebugInfo, nil, nil, err
	}
	treeCPBase, err := getTreeFromCommit(storage, commitHashCherryPickBase)
	if err != nil {
		return nil, fetchDebugInfo, nil, nil, err
	}
	treeCPTo, err := getTreeFromCommit(storage, commitHashCherryPickTo)
	if err != nil {
		return nil, fetchDebugInfo, nil, nil, err
	}

	collector := &conflictBlobCollector{}
	mergeResult, err := merge.MergeTree(storage, treeCPFrom, treeCPTo, treeCPBase, collector.Resolve)
	if err != nil {
		return nil, fetchDebugInfo, nil, nil, fmt.Errorf("failed to merge the trees: %v", err)
	}

	resolver := resolvediff3.NewDiff3Resolver(storage, "Cherry-pick content", "Base content", ".rej", "")
	var blobFetchDebugInfo *debug.FetchDebugInfo
	if len(mergeResult.FilesConflict) != 0 {
		// Need to fetch blobs and resolve the conflicts.
		if len(collector.blobHashes) > 0 {
			packfilebs, fetchBlobDebugInfo, err := fetch.FetchBlobPackfile(ctx, repoURL, client, collector.blobHashes)
			blobFetchDebugInfo = &fetchBlobDebugInfo
			if err != nil {
				return nil, fetchDebugInfo, blobFetchDebugInfo, nil, err
			}
			parser, err := packfile.NewParserWithStorage(packfile.NewScanner(bytes.NewReader(packfilebs)), storage)
			if err != nil {
				return nil, fetchDebugInfo, blobFetchDebugInfo, nil, fmt.Errorf("failed to parse packfile: %v", err)
			}
			if _, err := parser.Parse(); err != nil {
				return nil, fetchDebugInfo, blobFetchDebugInfo, nil, fmt.Errorf("failed to parse packfile: %v", err)
			}
		}
		mergeResult, err = merge.MergeTree(storage, treeCPFrom, treeCPTo, treeCPBase, resolver.Resolve)
		if err != nil {
			return nil, fetchDebugInfo, blobFetchDebugInfo, nil, fmt.Errorf("failed to merge the trees: %v", err)
		}
	}

	cpResult := &PushSquashCherryPickResult{
		CherryPickedFiles:     mergeResult.FilesPickedEntry1,
		ConflictOpenFiles:     resolver.ConflictOpenFiles,
		ConflictResolvedFiles: resolver.ConflictResolvedFiles,
		BinaryConflictFiles:   resolver.BinaryConflictFiles,
		NonFileConflictFiles:  resolver.NonFileConflictFiles,
	}
	hasConflict := len(cpResult.ConflictOpenFiles) > 0 || len(cpResult.BinaryConflictFiles) > 0 || len(cpResult.NonFileConflictFiles) > 0
	if abortOnConflict && hasConflict {
		return cpResult, fetchDebugInfo, blobFetchDebugInfo, nil, errors.New("conflict detected")
	}
	commit := &object.Commit{
		Message:      commitMessage,
		Author:       author,
		Committer:    committer,
		TreeHash:     mergeResult.TreeHash,
		ParentHashes: []plumbing.Hash{commitHashCherryPickTo},
	}
	obj := storage.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		return cpResult, fetchDebugInfo, blobFetchDebugInfo, nil, fmt.Errorf("failed to create a commit: %v", err)
	}
	commitHash, err := storage.SetEncodedObject(obj)
	if err != nil {
		return cpResult, fetchDebugInfo, blobFetchDebugInfo, nil, fmt.Errorf("failed to create a commit: %v", err)
	}
	cpResult.CommitHash = commitHash

	newHashes := make(map[plumbing.Hash]bool)
	newHashes[commitHash] = true
	for _, h := range mergeResult.NewHashes {
		newHashes[h] = true
	}
	for _, h := range resolver.NewHashes {
		newHashes[h] = true
	}

	var buf bytes.Buffer
	packEncoder := packfile.NewEncoder(&buf, storage, false)
	if _, err := packEncoder.Encode(slices.Collect(maps.Keys(newHashes)), 0); err != nil {
		return cpResult, fetchDebugInfo, blobFetchDebugInfo, nil, fmt.Errorf("failed to create a packfile: %v", err)
	}

	var destRef plumbing.ReferenceName
	if hasConflict && conflictRef != nil {
		destRef = *conflictRef
	} else {
		destRef = ref
	}

	pushDebugInfo, err := push.Push(ctx, repoURL, client, &buf, []push.RefUpdate{
		{
			Name:    destRef,
			OldHash: currentRefhash,
			NewHash: commitHash,
		},
	})
	if err != nil {
		return cpResult, fetchDebugInfo, blobFetchDebugInfo, &pushDebugInfo, err
	}
	return cpResult, fetchDebugInfo, blobFetchDebugInfo, &pushDebugInfo, nil
}

type conflictBlobCollector struct {
	blobHashes []plumbing.Hash
}

func (c *conflictBlobCollector) Resolve(parentPath string, entry1, entry2, entryBase *object.TreeEntry) ([]object.TreeEntry, error) {
	if entry1 != nil && entry1.Mode.IsFile() && entry2 != nil && entry2.Mode.IsFile() && entryBase != nil && entryBase.Mode.IsFile() {
		c.blobHashes = append(c.blobHashes, entry1.Hash, entry2.Hash, entryBase.Hash)
	}
	return nil, nil
}

func getTreeFromCommit(storage *memory.Storage, commitHash plumbing.Hash) (*object.Tree, error) {
	commit, err := object.GetCommit(storage, commitHash)
	if err != nil {
		return nil, fmt.Errorf("cannot find %q in the fetched packfile: %v", commitHash.String(), err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("cannot find the tree of %q in the fetched packfile: %v", commitHash.String(), err)
	}
	return tree, nil
}

func newSignature(name, email, timestamp string) (object.Signature, error) {
	var t time.Time
	if timestamp == "" {
		t = time.Now()
	} else {
		var err error
		t, err = time.Parse(time.RFC3339, timestamp)
		if err != nil {
			return object.Signature{}, err
		}
	}
	return object.Signature{
		Name:  name,
		Email: email,
		When:  t,
	}, nil
}
