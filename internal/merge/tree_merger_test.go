// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package merge

import (
	"io"
	"sort"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/go-cmp/cmp"
)

func TestMergeTree_Basic(t *testing.T) {
	// Basic file comparison patterns
	// /dir1/file1.txt A == Base && B == Base
	// /dir1/file2.txt A != Base && B == Base
	// /dir1/file3.txt A == Base && B != Base
	// /dir1/file4.txt A != Base && B != Base && A == B
	// /dir1/file5.txt A != Base && B != Base && A != B

	storage := memory.NewStorage()
	tree1, err := restoreTree(storage, dumpedTree{
		Dirs: map[string]dumpedTree{
			"dir1": {
				Files: map[string]string{
					"file1.txt": "Base",
					"file2.txt": "A",
					"file3.txt": "Base",
					"file4.txt": "AB",
					"file5.txt": "A",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tree2, err := restoreTree(storage, dumpedTree{
		Dirs: map[string]dumpedTree{
			"dir1": {
				Files: map[string]string{
					"file1.txt": "Base",
					"file2.txt": "Base",
					"file3.txt": "B",
					"file4.txt": "AB",
					"file5.txt": "B",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mergeBase, err := restoreTree(storage, dumpedTree{
		Dirs: map[string]dumpedTree{
			"dir1": {
				Files: map[string]string{
					"file1.txt": "Base",
					"file2.txt": "Base",
					"file3.txt": "Base",
					"file4.txt": "Base",
					"file5.txt": "Base",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := MergeTree(storage, tree1, tree2, mergeBase, testResolver)
	if err != nil {
		t.Fatal(err)
	}
	got, err := dumpTree(storage, result.TreeHash)
	if err != nil {
		t.Fatal(err)
	}
	want := dumpedTree{
		Files: map[string]string{},
		Dirs: map[string]dumpedTree{
			"dir1": {
				Files: map[string]string{
					"file1.txt":        "Base",
					"file2.txt":        "A",
					"file3.txt":        "B",
					"file4.txt":        "AB",
					"file5.txt.entry1": "A",
					"file5.txt.entry2": "B",
					"file5.txt.base":   "Base",
				},
				Dirs: map[string]dumpedTree{},
			},
		},
	}
	if !cmp.Equal(want, got) {
		t.Error("Got a diff\n" + cmp.Diff(want, got))
	}
}

func TestMergeTree_DirVsFile(t *testing.T) {
	// One side is a directory. The other side is a file. No recurse and pass it to the resolver.
	// /dir2/test A != Base && B is a dir
	// /dir2/test/file1.txt Exists only in B

	storage := memory.NewStorage()
	tree1, err := restoreTree(storage, dumpedTree{
		Dirs: map[string]dumpedTree{
			"dir2": {
				Files: map[string]string{
					"test": "A",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tree2, err := restoreTree(storage, dumpedTree{
		Dirs: map[string]dumpedTree{
			"dir2": {
				Dirs: map[string]dumpedTree{
					"test": {
						Files: map[string]string{
							"file1.txt": "B",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mergeBase, err := restoreTree(storage, dumpedTree{
		Dirs: map[string]dumpedTree{
			"dir2": {
				Files: map[string]string{
					"test": "Base",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := MergeTree(storage, tree1, tree2, mergeBase, testResolver)
	if err != nil {
		t.Fatal(err)
	}
	got, err := dumpTree(storage, result.TreeHash)
	if err != nil {
		t.Fatal(err)
	}
	want := dumpedTree{
		Files: map[string]string{},
		Dirs: map[string]dumpedTree{
			"dir2": {
				Files: map[string]string{
					"test.entry1": "A",
					"test.base":   "Base",
				},
				Dirs: map[string]dumpedTree{
					"test.entry2": {
						Files: map[string]string{
							"file1.txt": "B",
						},
						Dirs: map[string]dumpedTree{},
					},
				},
			},
		},
	}
	if !cmp.Equal(want, got) {
		t.Error("Got a diff\n" + cmp.Diff(want, got))
	}
}

func TestMergeTree_NoBaseDir(t *testing.T) {
	// Both sides are directories. Recurse without a base.
	// /dir3/test/file1.txt A != B && not in Base

	storage := memory.NewStorage()
	tree1, err := restoreTree(storage, dumpedTree{
		Dirs: map[string]dumpedTree{
			"dir3": {
				Dirs: map[string]dumpedTree{
					"test": {
						Files: map[string]string{
							"file1.txt": "A",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tree2, err := restoreTree(storage, dumpedTree{
		Dirs: map[string]dumpedTree{
			"dir3": {
				Dirs: map[string]dumpedTree{
					"test": {
						Files: map[string]string{
							"file1.txt": "B",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mergeBase, err := restoreTree(storage, dumpedTree{
		Files: map[string]string{
			"dummy": "Base",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := MergeTree(storage, tree1, tree2, mergeBase, testResolver)
	if err != nil {
		t.Fatal(err)
	}
	got, err := dumpTree(storage, result.TreeHash)
	if err != nil {
		t.Fatal(err)
	}
	want := dumpedTree{
		Files: map[string]string{},
		Dirs: map[string]dumpedTree{
			"dir3": {
				Files: map[string]string{},
				Dirs: map[string]dumpedTree{
					"test": {
						Files: map[string]string{
							"file1.txt.entry1": "A",
							"file1.txt.entry2": "B",
						},
						Dirs: map[string]dumpedTree{},
					},
				},
			},
		},
	}
	if !cmp.Equal(want, got) {
		t.Error("Got a diff\n" + cmp.Diff(want, got))
	}
}

func testResolver(parentPath string, entry1, entry2, base *object.TreeEntry) ([]object.TreeEntry, error) {
	var ret []object.TreeEntry
	if entry1 != nil {
		ret = append(ret, object.TreeEntry{Name: entry1.Name + ".entry1", Hash: entry1.Hash, Mode: entry1.Mode})
	}
	if entry2 != nil {
		ret = append(ret, object.TreeEntry{Name: entry2.Name + ".entry2", Hash: entry2.Hash, Mode: entry2.Mode})
	}
	if base != nil {
		ret = append(ret, object.TreeEntry{Name: base.Name + ".base", Hash: base.Hash, Mode: base.Mode})
	}
	return ret, nil
}

type dumpedTree struct {
	Files map[string]string
	Dirs  map[string]dumpedTree
}

func dumpTree(storage storer.EncodedObjectStorer, treehash plumbing.Hash) (dumpedTree, error) {
	var fn func(*object.Tree) (dumpedTree, error)
	fn = func(tree *object.Tree) (dumpedTree, error) {
		ret := dumpedTree{
			Files: make(map[string]string),
			Dirs:  make(map[string]dumpedTree),
		}
		for _, entry := range tree.Entries {
			if entry.Mode == filemode.Dir {
				subtree, err := object.GetTree(storage, entry.Hash)
				if err != nil {
					return dumpedTree{}, err
				}
				subtreeDump, err := fn(subtree)
				if err != nil {
					return dumpedTree{}, err
				}
				ret.Dirs[entry.Name] = subtreeDump
			} else {
				blob, err := object.GetBlob(storage, entry.Hash)
				if err != nil {
					return dumpedTree{}, err
				}
				s, err := readFullAsString(blob)
				if err != nil {
					return dumpedTree{}, err
				}
				ret.Files[entry.Name] = s
			}
		}
		return ret, nil
	}

	root, err := object.GetTree(storage, treehash)
	if err != nil {
		return dumpedTree{}, err
	}
	return fn(root)
}

func restoreTree(storage storer.EncodedObjectStorer, root dumpedTree) (*object.Tree, error) {
	var fn func(dumpedTree) (plumbing.Hash, error)
	fn = func(tree dumpedTree) (plumbing.Hash, error) {
		var entries []object.TreeEntry
		if tree.Files != nil {
			for name, content := range tree.Files {
				hash, err := createBlob(storage, content)
				if err != nil {
					return plumbing.ZeroHash, err
				}
				entries = append(entries, object.TreeEntry{Name: name, Hash: hash, Mode: filemode.Regular})
			}
		}
		if tree.Dirs != nil {
			for name, subtree := range tree.Dirs {
				subtreeHash, err := fn(subtree)
				if err != nil {
					return plumbing.ZeroHash, err
				}
				entries = append(entries, object.TreeEntry{Name: name, Hash: subtreeHash, Mode: filemode.Dir})
			}
		}
		sort.Sort(object.TreeEntrySorter(entries))
		newTree := object.Tree{Entries: entries}
		o := storage.NewEncodedObject()
		if err := newTree.Encode(o); err != nil {
			return plumbing.ZeroHash, err
		}
		newTreeHash, err := storage.SetEncodedObject(o)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		return newTreeHash, nil
	}

	rootTreeHash, err := fn(root)
	if err != nil {
		return nil, err
	}
	restoredRoot, err := object.GetTree(storage, rootTreeHash)
	if err != nil {
		return nil, err
	}
	return restoredRoot, nil
}

func readFullAsString(blob *object.Blob) (string, error) {
	rd, err := blob.Reader()
	if err != nil {
		return "", err
	}
	defer rd.Close()
	bs, err := io.ReadAll(rd)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func createBlob(storage storer.EncodedObjectStorer, content string) (plumbing.Hash, error) {
	o := storage.NewEncodedObject()
	o.SetType(plumbing.BlobObject)
	o.SetSize(int64(len(content)))

	wt, err := o.Writer()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if _, err := io.WriteString(wt, content); err != nil {
		wt.Close()
		return plumbing.ZeroHash, err
	}
	if err := wt.Close(); err != nil {
		return plumbing.ZeroHash, err
	}
	hash, err := storage.SetEncodedObject(o)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return hash, nil
}
