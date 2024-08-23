// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package resolvediff3

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"

	"github.com/epiclabs-io/diff3"
	"github.com/epiclabs-io/diff3/linereader"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

type Diff3Resolver struct {
	storage              storer.EncodedObjectStorer
	sideALabel           string
	sideBLabel           string
	sideARejectionSuffix string
	sideBRejectionSuffix string

	BinaryConflictFiles   []string
	NonFileConflictFiles  []string
	ConflictOpenFiles     []string
	ConflictResolvedFiles []string

	NewHashes []plumbing.Hash
}

func NewDiff3Resolver(storage storer.EncodedObjectStorer, sideALabel, sideBLabel, sideARejectionSuffix, sideBRejectionSuffix string) *Diff3Resolver {
	return &Diff3Resolver{
		storage:    storage,
		sideALabel: sideALabel,
		sideBLabel: sideBLabel,
	}
}

func (r *Diff3Resolver) Resolve(parentPath string, entry1, entry2, entryBase *object.TreeEntry) ([]object.TreeEntry, error) {
	// There are several cases, but we only have to deal with a case where both entry1 and
	// entry2, and entryBase are the files. Otherwise, we keep both entry1 and entry2.
	if entry1 != nil && entry1.Mode.IsFile() && entry2 != nil && entry2.Mode.IsFile() && entryBase != nil && entryBase.Mode.IsFile() {
		// Get the content of the files.
		blob1, err := r.storage.EncodedObject(plumbing.BlobObject, entry1.Hash)
		if err != nil {
			return nil, fmt.Errorf("cannot open the conflict file on side A %q: %v", entry1.Hash, err)
		}
		blob2, err := r.storage.EncodedObject(plumbing.BlobObject, entry2.Hash)
		if err != nil {
			return nil, fmt.Errorf("cannot open the conflict file on side B %q: %v", entry2.Hash, err)
		}
		blobBase, err := r.storage.EncodedObject(plumbing.BlobObject, entryBase.Hash)
		if err != nil {
			return nil, fmt.Errorf("cannot open the conflict file on base %q: %v", entry2.Hash, err)
		}

		resultHash, hasConflict, err := r.mergeDiff3(blob1, blob2, blobBase)
		if err != nil {
			if errors.Is(err, linereader.ErrBinaryContent) {
				// If the content is binary, we keep both files.
				r.BinaryConflictFiles = append(r.BinaryConflictFiles, path.Join(parentPath, entry1.Name))
				entry1.Name = entry1.Name + r.sideARejectionSuffix
				entry2.Name = entry2.Name + r.sideBRejectionSuffix
				return []object.TreeEntry{*entry1, *entry2}, nil
			}
			return nil, fmt.Errorf("cannot run diff3: %v", err)
		}
		entry := object.TreeEntry{
			Name: entry1.Name,
			Mode: entry1.Mode,
			Hash: resultHash,
		}
		if hasConflict {
			r.ConflictOpenFiles = append(r.ConflictOpenFiles, path.Join(parentPath, entry1.Name))
		} else {
			r.ConflictResolvedFiles = append(r.ConflictResolvedFiles, path.Join(parentPath, entry1.Name))
		}

		return []object.TreeEntry{entry}, nil
	}
	// Either entry1 or entry2 should be non-nil at this point (otherwise, it's not a conflict).
	if entry1 != nil {
		r.NonFileConflictFiles = append(r.NonFileConflictFiles, path.Join(parentPath, entry1.Name))
	} else if entry2 != nil {
		r.NonFileConflictFiles = append(r.NonFileConflictFiles, path.Join(parentPath, entry2.Name))
	}
	var ret []object.TreeEntry
	if entry1 != nil {
		entry1.Name = entry1.Name + r.sideARejectionSuffix
		ret = append(ret, *entry1)
	}
	if entry2 != nil {
		entry2.Name = entry2.Name + r.sideBRejectionSuffix
		ret = append(ret, *entry2)
	}
	return ret, nil
}

func (r *Diff3Resolver) mergeDiff3(blob1, blob2, blobBase plumbing.EncodedObject) (plumbing.Hash, bool, error) {
	rd1, err := blob1.Reader()
	if err != nil {
		return plumbing.ZeroHash, false, fmt.Errorf("cannot open blob1 as reader: %v", err)
	}
	defer rd1.Close()
	rd2, err := blob2.Reader()
	if err != nil {
		return plumbing.ZeroHash, false, fmt.Errorf("cannot open blob2 as reader: %v", err)
	}
	defer rd2.Close()
	rdBase, err := blobBase.Reader()
	if err != nil {
		return plumbing.ZeroHash, false, fmt.Errorf("cannot open blobBase as reader: %v", err)
	}
	defer rdBase.Close()

	mr, err := diff3.Merge(rd1, rdBase, rd2, false, r.sideALabel, r.sideBLabel)
	if err != nil {
		return plumbing.ZeroHash, false, fmt.Errorf("cannot run diff3 merge: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(mr.Result); err != nil {
		return plumbing.ZeroHash, false, fmt.Errorf("cannot read the diff3 merge result: %v", err)
	}
	// diff3 omits last "\n" in the result, so we add it.
	// https://github.com/epiclabs-io/diff3/blob/ba77e92bf0e4ce802f3fea4f6ba3aaf5f736bf3d/diff3.go#L529
	// https://stackoverflow.com/questions/729692/why-should-text-files-end-with-a-newline
	buf.WriteRune('\n')

	obj := r.storage.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	obj.SetSize(int64(buf.Len()))
	wt, err := obj.Writer()
	if err != nil {
		return plumbing.ZeroHash, false, fmt.Errorf("cannot open a blob writer: %v", err)
	}
	if _, err := io.Copy(wt, &buf); err != nil {
		return plumbing.ZeroHash, false, fmt.Errorf("cannot copy the merge result to a blob writer: %v", err)
	}
	if err := wt.Close(); err != nil {
		return plumbing.ZeroHash, false, fmt.Errorf("cannot close the blob writer: %v", err)
	}
	hash, err := r.storage.SetEncodedObject(obj)
	if err != nil {
		return plumbing.ZeroHash, false, fmt.Errorf("cannot save a blob: %v", err)
	}
	r.NewHashes = append(r.NewHashes, hash)
	return hash, mr.Conflicts, nil
}
