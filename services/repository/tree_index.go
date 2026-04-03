// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	git_model "github.com/gitjet-ru/core-scm/models/git"
	repo_model "github.com/gitjet-ru/core-scm/models/repo"
	"github.com/gitjet-ru/core-scm/modules/git/gitcmd"
	"github.com/gitjet-ru/core-scm/modules/gitrepo"
	"github.com/gitjet-ru/core-scm/modules/graceful"
	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/queue"
)

type BranchTreeIndexSyncOptions struct {
	RepoID    int64
	Branch    string
	CommitID  string
	Requested int64
}

var branchTreeIndexQueue *queue.WorkerPoolQueue[*BranchTreeIndexSyncOptions]

func initBranchTreeIndexQueue(ctx context.Context) error {
	branchTreeIndexQueue = queue.CreateUniqueQueue(ctx, "branch_tree_index_sync", handleBranchTreeIndexSync)
	if branchTreeIndexQueue == nil {
		return fmt.Errorf("unable to create branch_tree_index_sync queue")
	}
	go graceful.GetManager().RunWithCancel(branchTreeIndexQueue)
	return nil
}

func EnqueueBranchTreeIndexSync(repoID int64, branch, commitID string) error {
	if branchTreeIndexQueue == nil || strings.TrimSpace(branch) == "" || strings.TrimSpace(commitID) == "" {
		return nil
	}
	return branchTreeIndexQueue.Push(&BranchTreeIndexSyncOptions{
		RepoID:    repoID,
		Branch:    branch,
		CommitID:  commitID,
		Requested: time.Now().Unix(),
	})
}

func handleBranchTreeIndexSync(items ...*BranchTreeIndexSyncOptions) []*BranchTreeIndexSyncOptions {
	for _, item := range items {
		if err := RebuildBranchTreeIndex(graceful.GetManager().ShutdownContext(), item.RepoID, item.Branch, item.CommitID); err != nil {
			log.Error("RebuildBranchTreeIndex repo=%d branch=%s failed: %v", item.RepoID, item.Branch, err)
		}
	}
	return nil
}

func RebuildBranchTreeIndex(ctx context.Context, repoID int64, branch, commitID string) error {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(branch) == "" || strings.TrimSpace(commitID) == "" {
		return nil
	}

	cmd := gitcmd.NewCommand("ls-tree", "-rz").AddDynamicArguments(commitID)
	stdout, _, err := gitrepo.RunCmdString(ctx, repo, cmd)
	if err != nil {
		return err
	}

	version := time.Now().UnixNano()
	type key struct {
		dirPath string
		name    string
	}
	entryMap := map[key]*git_model.BranchTreeEntry{}
	addEntry := func(dirPath, name, mode, objectID, entryType string) {
		if name == "" {
			return
		}
		k := key{dirPath: strings.Trim(dirPath, "/"), name: name}
		if existing, ok := entryMap[k]; ok {
			// Keep file/blob info over inferred dir fallback.
			if existing.EntryType != "file" && existing.EntryType != "symlink" && existing.EntryType != "submodule" {
				existing.EntryMode = mode
				existing.ObjectID = objectID
				existing.EntryType = entryType
			}
			return
		}
		entryMap[k] = &git_model.BranchTreeEntry{
			RepoID:    repoID,
			Branch:    branch,
			Version:   version,
			DirPath:   strings.Trim(dirPath, "/"),
			EntryName: name,
			EntryMode: mode,
			ObjectID:  objectID,
			EntryType: entryType,
			Size:      0,
		}
	}

	records := strings.Split(stdout, "\x00")
	for _, rec := range records {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		parts := strings.SplitN(rec, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		meta := strings.Fields(parts[0]) // mode type sha
		if len(meta) < 3 {
			continue
		}
		mode, typ, objID := meta[0], meta[1], meta[2]
		fullPath := strings.Trim(parts[1], "/")
		if fullPath == "" {
			continue
		}
		segs := strings.Split(fullPath, "/")
		for i := 0; i < len(segs)-1; i++ {
			parent := strings.Join(segs[:i], "/")
			addEntry(parent, segs[i], "040000", "", "dir")
		}
		parent := ""
		if len(segs) > 1 {
			parent = strings.Join(segs[:len(segs)-1], "/")
		}
		entryType := "file"
		switch typ {
		case "tree":
			entryType = "dir"
		case "commit":
			entryType = "submodule"
		default:
			if mode == "120000" {
				entryType = "symlink"
			}
		}
		addEntry(parent, segs[len(segs)-1], mode, objID, entryType)
	}

	entries := make([]*git_model.BranchTreeEntry, 0, len(entryMap))
	for _, e := range entryMap {
		entries = append(entries, e)
	}

	idx := &git_model.BranchTreeIndex{
		RepoID:       repoID,
		Branch:       branch,
		HeadCommitID: commitID,
		Version:      version,
		State:        "ready",
		IndexedUnix:  time.Now().Unix(),
	}

	if err := git_model.UpsertBranchTreeIndex(ctx, idx); err != nil {
		return err
	}
	return git_model.ReplaceBranchTreeEntries(ctx, repoID, branch, version, entries)
}
