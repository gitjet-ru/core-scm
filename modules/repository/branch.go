// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gitjet-ru/core-scm/models/db"
	git_model "github.com/gitjet-ru/core-scm/models/git"
	repo_model "github.com/gitjet-ru/core-scm/models/repo"
	"github.com/gitjet-ru/core-scm/modules/container"
	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/gitjet-ru/core-scm/modules/git/gitcmd"
	"github.com/gitjet-ru/core-scm/modules/gitrepo"
	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/timeutil"
)

// SyncResult describes a reference update detected during sync.
type SyncResult struct {
	RefName     git.RefName
	OldCommitID string
	NewCommitID string
}

// SyncRepoBranches synchronizes branch table with repository branches
func SyncRepoBranches(ctx context.Context, repoID, doerID int64) (int64, error) {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return 0, err
	}

	log.Debug("SyncRepoBranches: in Repo[%d:%s]", repo.ID, repo.FullName())
	return syncRepoBranchesRemote(ctx, repo, doerID)
}

type remoteBranchMeta struct {
	commitID      string
	commitMessage string
	commitTime    timeutil.TimeStamp
}

func isRemoteGitStorageBackend() bool {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("GIT_STORAGE_BACKEND")))
	return backend == "remote" || backend == "shadow"
}

func syncRepoBranchesRemote(ctx context.Context, repo *repo_model.Repository, doerID int64) (int64, error) {
	out, _, runErr := gitrepo.RunCmdString(ctx, repo, gitcmd.NewCommand("for-each-ref",
		"--format=%(refname:short)%00%(objectname)%00%(subject)%00%(committerdate:unix)",
		"refs/heads",
	))
	if runErr != nil {
		return 0, runErr
	}

	allBranches := map[string]remoteBranchMeta{}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\x00")
		if len(parts) < 4 {
			continue
		}
		branchName := strings.TrimSpace(parts[0])
		commitID := strings.TrimSpace(parts[1])
		commitMessage := strings.TrimSpace(parts[2])
		unixTS, _ := strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
		allBranches[branchName] = remoteBranchMeta{
			commitID:      commitID,
			commitMessage: commitMessage,
			commitTime:    timeutil.TimeStamp(unixTS),
		}
	}

	dbBranches := make(map[string]*git_model.Branch)
	branches, err := db.Find[git_model.Branch](ctx, git_model.FindBranchOptions{
		ListOptions: db.ListOptionsAll,
		RepoID:      repo.ID,
	})
	if err != nil {
		return 0, err
	}
	for _, branch := range branches {
		dbBranches[branch.Name] = branch
	}

	var toAdd []*git_model.Branch
	var toUpdate []*git_model.Branch
	var toRemove []int64
	for branchName, meta := range allBranches {
		dbb := dbBranches[branchName]
		if dbb == nil {
			toAdd = append(toAdd, &git_model.Branch{
				RepoID:        repo.ID,
				Name:          branchName,
				CommitID:      meta.commitID,
				CommitMessage: meta.commitMessage,
				PusherID:      doerID,
				CommitTime:    meta.commitTime,
			})
			continue
		}
		if dbb.CommitID != meta.commitID || dbb.IsDeleted {
			toUpdate = append(toUpdate, &git_model.Branch{
				ID:            dbb.ID,
				RepoID:        repo.ID,
				Name:          branchName,
				CommitID:      meta.commitID,
				CommitMessage: meta.commitMessage,
				PusherID:      doerID,
				CommitTime:    meta.commitTime,
			})
		}
	}

	for _, dbBranch := range dbBranches {
		if _, ok := allBranches[dbBranch.Name]; !ok && !dbBranch.IsDeleted {
			toRemove = append(toRemove, dbBranch.ID)
		}
	}

	if len(toAdd) == 0 && len(toRemove) == 0 && len(toUpdate) == 0 {
		return int64(len(allBranches)), nil
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if len(toAdd) > 0 {
			if err := git_model.AddBranches(ctx, toAdd); err != nil {
				return err
			}
		}
		for _, b := range toUpdate {
			if _, err := db.GetEngine(ctx).ID(b.ID).
				Cols("commit_id, commit_message, pusher_id, commit_time, is_deleted").
				Update(b); err != nil {
				return err
			}
		}
		if len(toRemove) > 0 {
			if err := git_model.DeleteBranches(ctx, repo.ID, doerID, toRemove); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}

	return int64(len(allBranches)), nil
}

func SyncRepoBranchesWithRepo(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, doerID int64) (int64, []*SyncResult, error) {
	objFmt, err := gitRepo.GetObjectFormat()
	if err != nil {
		return 0, nil, fmt.Errorf("GetObjectFormat: %w", err)
	}
	if objFmt.Name() != repo.ObjectFormatName {
		repo.ObjectFormatName = objFmt.Name()
		if err = repo_model.UpdateRepositoryColsWithAutoTime(ctx, repo, "object_format_name"); err != nil {
			return 0, nil, fmt.Errorf("UpdateRepositoryColsWithAutoTime: %w", err)
		}
	}

	allBranches := container.Set[string]{}
	{
		branches, _, err := gitRepo.GetBranchNames(0, 0)
		if err != nil {
			return 0, nil, err
		}
		log.Trace("SyncRepoBranches[%s]: branches[%d]: %v", repo.FullName(), len(branches), branches)
		for _, branch := range branches {
			allBranches.Add(branch)
		}
	}

	dbBranches := make(map[string]*git_model.Branch)
	{
		branches, err := db.Find[git_model.Branch](ctx, git_model.FindBranchOptions{
			ListOptions: db.ListOptionsAll,
			RepoID:      repo.ID,
		})
		if err != nil {
			return 0, nil, err
		}
		for _, branch := range branches {
			dbBranches[branch.Name] = branch
		}
	}

	var toAdd []*git_model.Branch
	var toUpdate []*git_model.Branch
	var toRemove []int64
	var syncResults []*SyncResult
	for branch := range allBranches {
		dbb := dbBranches[branch]
		commit, err := gitRepo.GetBranchCommit(branch)
		if err != nil {
			return 0, nil, err
		}
		if dbb == nil {
			toAdd = append(toAdd, &git_model.Branch{
				RepoID:        repo.ID,
				Name:          branch,
				CommitID:      commit.ID.String(),
				CommitMessage: commit.Summary(),
				PusherID:      doerID,
				CommitTime:    timeutil.TimeStamp(commit.Committer.When.Unix()),
			})
			syncResults = append(syncResults, &SyncResult{
				RefName:     git.RefNameFromBranch(branch),
				OldCommitID: "",
				NewCommitID: commit.ID.String(),
			})
		} else if commit.ID.String() != dbb.CommitID || dbb.IsDeleted {
			toUpdate = append(toUpdate, &git_model.Branch{
				ID:            dbb.ID,
				RepoID:        repo.ID,
				Name:          branch,
				CommitID:      commit.ID.String(),
				CommitMessage: commit.Summary(),
				PusherID:      doerID,
				CommitTime:    timeutil.TimeStamp(commit.Committer.When.Unix()),
			})
			syncResults = append(syncResults, &SyncResult{
				RefName:     git.RefNameFromBranch(branch),
				OldCommitID: dbb.CommitID,
				NewCommitID: commit.ID.String(),
			})
		}
	}

	for _, dbBranch := range dbBranches {
		if !allBranches.Contains(dbBranch.Name) && !dbBranch.IsDeleted {
			toRemove = append(toRemove, dbBranch.ID)
			syncResults = append(syncResults, &SyncResult{
				RefName:     git.RefNameFromBranch(dbBranch.Name),
				OldCommitID: dbBranch.CommitID,
				NewCommitID: "",
			})
		}
	}

	log.Trace("SyncRepoBranches[%s]: toAdd: %v, toUpdate: %v, toRemove: %v", repo.FullName(), toAdd, toUpdate, toRemove)

	if len(toAdd) == 0 && len(toRemove) == 0 && len(toUpdate) == 0 {
		return int64(len(allBranches)), syncResults, nil
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if len(toAdd) > 0 {
			if err := git_model.AddBranches(ctx, toAdd); err != nil {
				return err
			}
		}

		for _, b := range toUpdate {
			if _, err := db.GetEngine(ctx).ID(b.ID).
				Cols("commit_id, commit_message, pusher_id, commit_time, is_deleted").
				Update(b); err != nil {
				return err
			}
		}

		if len(toRemove) > 0 {
			if err := git_model.DeleteBranches(ctx, repo.ID, doerID, toRemove); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return 0, nil, err
	}
	return int64(len(allBranches)), syncResults, nil
}
