// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"io"
	"sort"
	"strings"

	"github.com/gitjet-ru/core-scm/modules/git/gitcmd"
)

// Tree represents a flat directory listing.
type Tree struct {
	TreeCommon

	entries       Entries
	entriesParsed bool
}

// ListEntries returns all entries of current tree.
func (t *Tree) ListEntries() (Entries, error) {
	if t.entriesParsed {
		return t.entries, nil
	}

	// Mirrorless web reads: fetch tree entries directly from git-storage.
	if remoteReadRepo(t.repo) {
		entries, _, err := remoteGetTree(t.repo.Ctx, t.repo.Path, t.ID.String(), "", false, 0)
		if err != nil {
			return nil, err
		}

		// Ensure deterministic ordering (ls-tree is stable but RPC parsing can vary).
		sort.Slice(entries, func(i, j int) bool { return entries[i].GetPath() < entries[j].GetPath() })

		ret := make(Entries, 0, len(entries))
		for _, e := range entries {
			if e == nil {
				continue
			}
			mode := ParseEntryMode(e.GetMode())
			id, err := NewIDFromString(e.GetObjectId())
			if err != nil {
				return nil, err
			}

			te := &TreeEntry{
				ID:        id,
				ptree:     t,
				name:      e.GetPath(),
				entryMode: mode,
			}
			// Blob-ish entries include size in ls-tree output; mark them as sized so Blob.Size doesn't need cat-file.
			if e.GetObjectType() == string(ObjectBlob) || mode != EntryModeTree && mode != EntryModeCommit {
				// e.Size might be 0 for some edge cases; that's still fine.
				te.size = e.GetSize()
				te.sized = true
			} else if mode == EntryModeTree || mode == EntryModeCommit {
				// directory/submodule: size isn't a blob size
				te.size = 0
			}
			ret = append(ret, te)
		}

		t.entries = ret
		t.entriesParsed = true
		return t.entries, nil
	}

	if t.repo != nil {
		batch, cancel, err := t.repo.CatFileBatch(t.repo.Ctx)
		if err != nil {
			return nil, err
		}
		defer cancel()

		info, rd, err := batch.QueryContent(t.ID.String())
		if err != nil {
			return nil, err
		}

		if info.Type == "commit" {
			treeID, err := ReadTreeID(rd, info.Size)
			if err != nil && err != io.EOF {
				return nil, err
			}
			info, rd, err = batch.QueryContent(treeID)
			if err != nil {
				return nil, err
			}
		}
		if info.Type == "tree" {
			t.entries, err = catBatchParseTreeEntries(t.ID.Type(), t, rd, info.Size)
			if err != nil {
				return nil, err
			}
			t.entriesParsed = true
			return t.entries, nil
		}

		// Not a tree just use ls-tree instead
		if err := DiscardFull(rd, info.Size+1); err != nil {
			return nil, err
		}
	}

	stdout, _, runErr := gitcmd.NewCommand("ls-tree", "-l").AddDynamicArguments(t.ID.String()).WithDir(t.repo.Path).RunStdBytes(t.repo.Ctx)
	if runErr != nil {
		if gitcmd.IsStdErrorNotValidObjectName(runErr) || strings.Contains(runErr.Error(), "fatal: not a tree object") {
			return nil, ErrNotExist{
				ID: t.ID.String(),
			}
		}
		return nil, runErr
	}

	var err error
	t.entries, err = parseTreeEntries(stdout, t)
	if err == nil {
		t.entriesParsed = true
	}

	return t.entries, err
}

// listEntriesRecursive returns all entries of current tree recursively including all subtrees
// extraArgs could be "-l" to get the size, which is slower
func (t *Tree) listEntriesRecursive(extraArgs gitcmd.TrustedCmdArgs) (Entries, error) {
	stdout, _, runErr := gitcmd.NewCommand("ls-tree", "-t", "-r").
		AddArguments(extraArgs...).
		AddDynamicArguments(t.ID.String()).
		WithDir(t.repo.Path).
		RunStdBytes(t.repo.Ctx)
	if runErr != nil {
		return nil, runErr
	}

	// FIXME: the "name" field is abused, here it is a full path
	// FIXME: this ptree is not right, fortunately it isn't really used
	return parseTreeEntries(stdout, t)
}

// ListEntriesRecursiveFast returns all entries of current tree recursively including all subtrees, no size
func (t *Tree) ListEntriesRecursiveFast() (Entries, error) {
	return t.listEntriesRecursive(nil)
}

// ListEntriesRecursiveWithSize returns all entries of current tree recursively including all subtrees, with size
func (t *Tree) ListEntriesRecursiveWithSize() (Entries, error) {
	return t.listEntriesRecursive(gitcmd.TrustedCmdArgs{"--long"})
}
