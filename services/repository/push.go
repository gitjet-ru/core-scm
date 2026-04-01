// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gitjet-ru/core-scm/models/db"
	repo_model "github.com/gitjet-ru/core-scm/models/repo"
	user_model "github.com/gitjet-ru/core-scm/models/user"
	"github.com/gitjet-ru/core-scm/modules/cache"
	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/gitjet-ru/core-scm/modules/gitrepo"
	"github.com/gitjet-ru/core-scm/modules/graceful"
	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/process"
	"github.com/gitjet-ru/core-scm/modules/queue"
	repo_module "github.com/gitjet-ru/core-scm/modules/repository"
	"github.com/gitjet-ru/core-scm/modules/setting"
	"github.com/gitjet-ru/core-scm/modules/timeutil"
	notify_service "github.com/gitjet-ru/core-scm/services/notify"
	pull_service "github.com/gitjet-ru/core-scm/services/pull"
)

// pushQueue represents a queue to handle update pull request tests
var pushQueue *queue.WorkerPoolQueue[[]*repo_module.PushUpdateOptions]

// handle passed PR IDs and test the PRs
func handler(items ...[]*repo_module.PushUpdateOptions) [][]*repo_module.PushUpdateOptions {
	for _, opts := range items {
		if err := pushUpdates(opts); err != nil {
			// Username and repository stays the same between items in opts.
			pushUpdate := opts[0]
			log.Error("pushUpdate[%s/%s] failed: %v", pushUpdate.RepoUserName, pushUpdate.RepoName, err)
		}
	}
	return nil
}

func initPushQueue() error {
	pushQueue = queue.CreateSimpleQueue(graceful.GetManager().ShutdownContext(), "push_update", handler)
	if pushQueue == nil {
		return errors.New("unable to create push_update queue")
	}
	go graceful.GetManager().RunWithCancel(pushQueue)
	return nil
}

// PushUpdate is an alias of PushUpdates for single push update options
func PushUpdate(opts *repo_module.PushUpdateOptions) error {
	return PushUpdates([]*repo_module.PushUpdateOptions{opts})
}

// PushUpdates adds a push update to push queue
func PushUpdates(opts []*repo_module.PushUpdateOptions) error {
	if len(opts) == 0 {
		return nil
	}

	for _, opt := range opts {
		if opt.IsNewRef() && opt.IsDelRef() {
			return errors.New("Old and new revisions are both NULL")
		}
	}

	return pushQueue.Push(opts)
}

// pushUpdates generates push action history feeds for push updating multiple refs
func pushUpdates(optsList []*repo_module.PushUpdateOptions) error {
	if len(optsList) == 0 {
		return nil
	}

	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("PushUpdates: %s/%s", optsList[0].RepoUserName, optsList[0].RepoName))
	defer finished()

	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, optsList[0].RepoUserName, optsList[0].RepoName)
	if err != nil {
		return fmt.Errorf("GetRepositoryByOwnerAndName failed: %w", err)
	}
	log.Warn("skip pushUpdates mirror-dependent flow in mirrorless hard-cut for %s", repo.RelativePath())
	_ = repo_module.UpdateRepoSize(ctx, repo)
	_ = repo_model.UpdateRepositoryUpdatedTime(ctx, repo.ID, time.Now())
	return nil
}

func getCompareURL(repo *repo_model.Repository, gitRepo *git.Repository, objectFormat git.ObjectFormat, commits []*repo_module.PushCommit, opts *repo_module.PushUpdateOptions) string {
	oldCommitID := opts.OldCommitID
	if oldCommitID == objectFormat.EmptyObjectID().String() && len(commits) > 0 {
		oldCommit, err := gitRepo.GetCommit(commits[len(commits)-1].Sha1)
		if err != nil && !git.IsErrNotExist(err) {
			log.Error("unable to GetCommit %s from %-v: %v", oldCommitID, repo, err)
		}
		if oldCommit != nil {
			for i := 0; i < oldCommit.ParentCount(); i++ {
				commitID, _ := oldCommit.ParentID(i)
				if !commitID.IsZero() {
					oldCommitID = commitID.String()
					break
				}
			}
		}
	}

	if oldCommitID == objectFormat.EmptyObjectID().String() && repo.DefaultBranch != opts.RefFullName.BranchName() {
		oldCommitID = repo.DefaultBranch
	}

	if oldCommitID != objectFormat.EmptyObjectID().String() {
		return repo.ComposeCompareURL(oldCommitID, opts.NewCommitID)
	}
	return ""
}

func pushNewBranch(ctx context.Context, repo *repo_model.Repository, pusher *user_model.User, opts *repo_module.PushUpdateOptions, newCommit *git.Commit) ([]*git.Commit, error) {
	if repo.IsEmpty { // Change default branch and empty status only if pushed ref is non-empty branch.
		repo.DefaultBranch = opts.RefName()
		repo.IsEmpty = false
		if repo.DefaultBranch != setting.Repository.DefaultBranch {
			if err := gitrepo.SetDefaultBranch(ctx, repo, repo.DefaultBranch); err != nil {
				return nil, err
			}
		}
		// Update the is empty and default_branch columns
		if err := repo_model.UpdateRepositoryColsWithAutoTime(ctx, repo, "default_branch", "is_empty"); err != nil {
			return nil, fmt.Errorf("UpdateRepositoryCols: %w", err)
		}
	}

	l, err := newCommit.CommitsBeforeLimit(10)
	if err != nil {
		return nil, fmt.Errorf("newCommit.CommitsBeforeLimit: %w", err)
	}
	notify_service.CreateRef(ctx, pusher, repo, opts.RefFullName, opts.NewCommitID)
	return l, nil
}

func pushUpdateBranch(_ context.Context, repo *repo_model.Repository, pusher *user_model.User, opts *repo_module.PushUpdateOptions, newCommit *git.Commit) ([]*git.Commit, error) {
	l, err := newCommit.CommitsBeforeUntil(opts.OldCommitID)
	if err != nil {
		return nil, fmt.Errorf("newCommit.CommitsBeforeUntil: %w", err)
	}

	branch := opts.RefFullName.BranchName()

	isForcePush, err := newCommit.IsForcePush(opts.OldCommitID)
	if err != nil {
		log.Error("IsForcePush %s:%s failed: %v", repo.FullName(), branch, err)
	}

	// only update branch can trigger pull request task because the pull request hasn't been created yet when creating a branch
	go pull_service.AddTestPullRequestTask(pull_service.TestPullRequestOptions{
		RepoID:      repo.ID,
		Doer:        pusher,
		Branch:      branch,
		IsSync:      true,
		IsForcePush: isForcePush,
		OldCommitID: opts.OldCommitID,
		NewCommitID: opts.NewCommitID,
	})

	if isForcePush {
		log.Trace("Push %s is a force push", opts.NewCommitID)

		cache.Remove(repo.GetCommitsCountCacheKey(opts.RefName(), true))
	} else {
		// TODO: increment update the commit count cache but not remove
		cache.Remove(repo.GetCommitsCountCacheKey(opts.RefName(), true))
	}

	return l, nil
}

func pushDeleteBranch(ctx context.Context, repo *repo_model.Repository, pusher *user_model.User, opts *repo_module.PushUpdateOptions) {
	notify_service.DeleteRef(ctx, pusher, repo, opts.RefFullName)

	if err := pull_service.AdjustPullsCausedByBranchDeleted(ctx, pusher, repo, opts.RefFullName.BranchName()); err != nil {
		// close all related pulls
		log.Error("close related pull request failed: %v", err)
	}
}

// PushUpdateAddDeleteTags updates a number of added and delete tags
func PushUpdateAddDeleteTags(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, pusher *user_model.User, addTags, delTags []string) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := repo_model.PushUpdateDeleteTags(ctx, repo, delTags); err != nil {
			return err
		}
		return pushUpdateAddTags(ctx, repo, gitRepo, pusher, addTags)
	})
}

// pushUpdateAddTags updates a number of add tags
func pushUpdateAddTags(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, pusher *user_model.User, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	releases, err := db.Find[repo_model.Release](ctx, repo_model.FindReleasesOptions{
		RepoID:        repo.ID,
		TagNames:      tags,
		IncludeDrafts: true,
		IncludeTags:   true,
	})
	if err != nil {
		return fmt.Errorf("db.Find[repo_model.Release]: %w", err)
	}
	relMap := make(map[string]*repo_model.Release)
	for _, rel := range releases {
		relMap[rel.LowerTagName] = rel
	}

	lowerTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		lowerTags = append(lowerTags, strings.ToLower(tag))
	}

	newReleases := make([]*repo_model.Release, 0, len(lowerTags)-len(relMap))

	for i, lowerTag := range lowerTags {
		tag, err := gitRepo.GetTag(tags[i])
		if err != nil {
			return fmt.Errorf("GetTag: %w", err)
		}
		commit, err := gitRepo.GetTagCommit(tag.Name)
		if err != nil {
			return fmt.Errorf("Commit: %w", err)
		}

		sig := tag.Tagger
		if sig == nil {
			sig = commit.Author
		}
		if sig == nil {
			sig = commit.Committer
		}

		createdAt := time.Unix(1, 0)
		if sig != nil {
			createdAt = sig.When
		}

		rel, has := relMap[lowerTag]
		title, note := git.SplitCommitTitleBody(tag.Message, 255)
		if !has {
			rel = &repo_model.Release{
				RepoID:       repo.ID,
				Title:        title,
				TagName:      tags[i],
				LowerTagName: lowerTag,
				Target:       "",
				Sha1:         commit.ID.String(),
				NumCommits:   -1, // the commits count will be updated when the UI needs it
				Note:         note,
				IsDraft:      false,
				IsPrerelease: false,
				IsTag:        true,
				PublisherID:  pusher.ID,
				CreatedUnix:  timeutil.TimeStamp(createdAt.Unix()),
			}

			newReleases = append(newReleases, rel)
		} else {
			rel.Sha1 = commit.ID.String()
			rel.CreatedUnix = timeutil.TimeStamp(createdAt.Unix())
			if rel.IsTag {
				rel.Title = title
				rel.Note = note
			} else {
				rel.IsDraft = false
			}
			rel.PublisherID = pusher.ID
			if err = repo_model.UpdateRelease(ctx, rel); err != nil {
				return fmt.Errorf("Update: %w", err)
			}
		}
	}

	if len(newReleases) > 0 {
		if err = db.Insert(ctx, newReleases); err != nil {
			return fmt.Errorf("Insert: %w", err)
		}
	}

	return nil
}
