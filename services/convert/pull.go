// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"fmt"

	git_model "github.com/gitjet-ru/core-scm/models/git"
	issues_model "github.com/gitjet-ru/core-scm/models/issues"
	"github.com/gitjet-ru/core-scm/models/perm"
	access_model "github.com/gitjet-ru/core-scm/models/perm/access"
	repo_model "github.com/gitjet-ru/core-scm/models/repo"
	user_model "github.com/gitjet-ru/core-scm/models/user"
	"github.com/gitjet-ru/core-scm/modules/cache"
	"github.com/gitjet-ru/core-scm/modules/cachegroup"
	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/gitjet-ru/core-scm/modules/gitrepo"
	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/setting"
	api "github.com/gitjet-ru/core-scm/modules/structs"
	"github.com/gitjet-ru/core-scm/modules/util"
	"github.com/gitjet-ru/core-scm/services/gitdiff"
)

// ToAPIPullRequest assumes following fields have been assigned with valid values:
// Required - Issue
// Optional - Merger
func ToAPIPullRequest(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User) *api.PullRequest {
	var (
		err        error
	)

	if err = pr.LoadIssue(ctx); err != nil {
		log.Error("pr.LoadIssue[%d]: %v", pr.ID, err)
		return nil
	}

	if err = pr.Issue.LoadRepo(ctx); err != nil {
		log.Error("pr.Issue.LoadRepo[%d]: %v", pr.ID, err)
		return nil
	}

	apiIssue := ToAPIIssue(ctx, doer, pr.Issue)
	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("GetRepositoryById[%d]: %v", pr.ID, err)
		return nil
	}

	if err := pr.LoadHeadRepo(ctx); err != nil {
		log.Error("GetRepositoryById[%d]: %v", pr.ID, err)
		return nil
	}

	var doerID int64
	if doer != nil {
		doerID = doer.ID
	}

	repoUserPerm, err := cache.GetWithContextCache(ctx, cachegroup.RepoUserPermission, fmt.Sprintf("%d-%d", pr.BaseRepoID, doerID),
		func(ctx context.Context, _ string) (access_model.Permission, error) {
			return access_model.GetUserRepoPermission(ctx, pr.BaseRepo, doer)
		},
	)
	if err != nil {
		log.Error("GetUserRepoPermission[%d]: %v", pr.BaseRepoID, err)
		repoUserPerm.AccessMode = perm.AccessModeNone
	}

	apiPullRequest := &api.PullRequest{
		ID:             pr.ID,
		URL:            pr.Issue.HTMLURL(ctx),
		Index:          pr.Index,
		Poster:         apiIssue.Poster,
		Title:          apiIssue.Title,
		Body:           apiIssue.Body,
		Labels:         apiIssue.Labels,
		Milestone:      apiIssue.Milestone,
		Assignee:       apiIssue.Assignee,
		Assignees:      util.SliceNilAsEmpty(apiIssue.Assignees),
		State:          apiIssue.State,
		Draft:          pr.IsWorkInProgress(ctx),
		IsLocked:       apiIssue.IsLocked,
		Comments:       apiIssue.Comments,
		ReviewComments: pr.GetReviewCommentsCount(ctx),
		HTMLURL:        pr.Issue.HTMLURL(ctx),
		DiffURL:        pr.Issue.DiffURL(),
		PatchURL:       pr.Issue.PatchURL(),
		HasMerged:      pr.HasMerged,
		MergeBase:      pr.MergeBase,
		Mergeable:      pr.Mergeable(ctx),
		Deadline:       apiIssue.Deadline,
		Created:        pr.Issue.CreatedUnix.AsTimePtr(),
		Updated:        pr.Issue.UpdatedUnix.AsTimePtr(),
		PinOrder:       util.Iif(apiIssue.PinOrder == -1, 0, apiIssue.PinOrder),

		// output "[]" rather than null to align to github outputs
		RequestedReviewers:      []*api.User{},
		RequestedReviewersTeams: []*api.Team{},

		AllowMaintainerEdit: pr.AllowMaintainerEdit,

		Base: &api.PRBranchInfo{
			Name:       pr.BaseBranch,
			Ref:        pr.BaseBranch,
			RepoID:     pr.BaseRepoID,
			Repository: ToRepo(ctx, pr.BaseRepo, repoUserPerm),
		},
		Head: &api.PRBranchInfo{
			Name:   pr.HeadBranch,
			Ref:    pr.GetGitHeadRefName(),
			RepoID: -1,
		},
	}

	if err = pr.LoadRequestedReviewers(ctx); err != nil {
		log.Error("LoadRequestedReviewers[%d]: %v", pr.ID, err)
		return nil
	}
	if err = pr.LoadRequestedReviewersTeams(ctx); err != nil {
		log.Error("LoadRequestedReviewersTeams[%d]: %v", pr.ID, err)
		return nil
	}

	for _, reviewer := range pr.RequestedReviewers {
		apiPullRequest.RequestedReviewers = append(apiPullRequest.RequestedReviewers, ToUser(ctx, reviewer, nil))
	}

	for _, reviewerTeam := range pr.RequestedReviewersTeams {
		convertedTeam, err := ToTeam(ctx, reviewerTeam, true)
		if err != nil {
			log.Error("LoadRequestedReviewersTeams[%d]: %v", pr.ID, err)
			return nil
		}

		apiPullRequest.RequestedReviewersTeams = append(apiPullRequest.RequestedReviewersTeams, convertedTeam)
	}

	if pr.Issue.ClosedUnix != 0 {
		apiPullRequest.Closed = pr.Issue.ClosedUnix.AsTimePtr()
	}

	exist, err := git_model.IsBranchExist(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		log.Error("GetBranch[%s]: %v", pr.BaseBranch, err)
		return nil
	}

	if exist {
		baseBranchModel, branchErr := git_model.GetBranch(ctx, pr.BaseRepoID, pr.BaseBranch)
		if branchErr != nil && !git_model.IsErrBranchNotExist(branchErr) {
			log.Error("GetBranch[%s]: %v", pr.BaseBranch, branchErr)
			return nil
		}
		if branchErr == nil {
			apiPullRequest.Base.Sha = baseBranchModel.CommitID
		}
	}

	if pr.Flow == issues_model.PullRequestFlowAGit {
		apiPullRequest.Head.Sha, err = gitrepo.GetFullCommitID(ctx, pr.BaseRepo, pr.GetGitHeadRefName())
		if err != nil {
			log.Error("GetRefCommitID[%s]: %v", pr.GetGitHeadRefName(), err)
			return nil
		}
		apiPullRequest.Head.RepoID = pr.BaseRepoID
		apiPullRequest.Head.Repository = apiPullRequest.Base.Repository
		apiPullRequest.Head.Name = ""
	}

	if pr.HeadRepo != nil && pr.Flow == issues_model.PullRequestFlowGithub {
		p, err := access_model.GetUserRepoPermission(ctx, pr.HeadRepo, doer)
		if err != nil {
			log.Error("GetUserRepoPermission[%d]: %v", pr.HeadRepoID, err)
			p.AccessMode = perm.AccessModeNone
		}

		apiPullRequest.Head.RepoID = pr.HeadRepo.ID
		apiPullRequest.Head.Repository = ToRepo(ctx, pr.HeadRepo, p)

		exist, err = git_model.IsBranchExist(ctx, pr.HeadRepoID, pr.HeadBranch)
		if err != nil {
			log.Error("GetBranch[%s]: %v", pr.HeadBranch, err)
			return nil
		}

		// Outer scope variables to be used in diff calculation
		var (
			startCommitID string
			endCommitID   string
		)

		if !exist {
			headCommitID, err := gitrepo.GetFullCommitID(ctx, pr.HeadRepo, apiPullRequest.Head.Ref)
			if err != nil && !git.IsErrNotExist(err) {
				log.Error("GetCommit[%s]: %v", pr.HeadBranch, err)
				return nil
			}
			if err == nil {
				apiPullRequest.Head.Sha = headCommitID
				endCommitID = headCommitID
			}
		} else {
			headCommitID, err := gitrepo.GetFullCommitID(ctx, pr.HeadRepo, pr.HeadBranch)
			if err != nil && !git.IsErrNotExist(err) {
				log.Error("GetCommit[%s]: %v", pr.HeadBranch, err)
				return nil
			}
			if err == nil {
				apiPullRequest.Head.Ref = pr.HeadBranch
				apiPullRequest.Head.Sha = headCommitID
				endCommitID = headCommitID
			}
		}

		// Calculate diff
		startCommitID = pr.MergeBase
		diffShortStats, err := gitdiff.GetDiffShortStatByIDs(ctx, pr.BaseRepo, startCommitID, endCommitID)
		if err != nil {
			log.Error("GetDiffShortStat: %v", err)
		} else {
			apiPullRequest.ChangedFiles = &diffShortStats.NumFiles
			apiPullRequest.Additions = &diffShortStats.TotalAddition
			apiPullRequest.Deletions = &diffShortStats.TotalDeletion
		}
	}

	if len(apiPullRequest.Head.Sha) == 0 && len(apiPullRequest.Head.Ref) != 0 {
		headSHA, err := gitrepo.GetFullCommitID(ctx, pr.BaseRepo, apiPullRequest.Head.Ref)
		if err != nil && !git.IsErrNotExist(err) {
			log.Error("GetRefCommitID[%s]: %v", apiPullRequest.Head.Ref, err)
			return nil
		}
		if err == nil {
			apiPullRequest.Head.Sha = headSHA
		}
	}

	if pr.HasMerged {
		apiPullRequest.Merged = pr.MergedUnix.AsTimePtr()
		apiPullRequest.MergedCommitID = &pr.MergedCommitID
		apiPullRequest.MergedBy = ToUser(ctx, pr.Merger, nil)
	}

	return apiPullRequest
}

func ToAPIPullRequests(ctx context.Context, baseRepo *repo_model.Repository, prs issues_model.PullRequestList, doer *user_model.User) ([]*api.PullRequest, error) {
	for _, pr := range prs {
		pr.BaseRepo = baseRepo
		if pr.BaseRepoID == pr.HeadRepoID {
			pr.HeadRepo = baseRepo
		}
	}

	// NOTE: load head repositories
	if err := prs.LoadRepositories(ctx); err != nil {
		return nil, err
	}
	issueList, err := prs.LoadIssues(ctx)
	if err != nil {
		return nil, err
	}

	if err := issueList.LoadLabels(ctx); err != nil {
		return nil, err
	}
	if err := issueList.LoadPosters(ctx); err != nil {
		return nil, err
	}
	if err := issueList.LoadAttachments(ctx); err != nil {
		return nil, err
	}
	if err := issueList.LoadMilestones(ctx); err != nil {
		return nil, err
	}
	if err := issueList.LoadAssignees(ctx); err != nil {
		return nil, err
	}
	if err = issueList.LoadPinOrder(ctx); err != nil {
		return nil, err
	}

	reviews, err := prs.LoadReviews(ctx)
	if err != nil {
		return nil, err
	}
	if err = reviews.LoadReviewers(ctx); err != nil {
		return nil, err
	}

	reviewersMap := make(map[int64][]*user_model.User)
	for _, review := range reviews {
		if review.Reviewer != nil {
			reviewersMap[review.IssueID] = append(reviewersMap[review.IssueID], review.Reviewer)
		}
	}

	reviewCounts, err := prs.LoadReviewCommentsCounts(ctx)
	if err != nil {
		return nil, err
	}

	baseRepoPerm, err := access_model.GetUserRepoPermission(ctx, baseRepo, doer)
	if err != nil {
		log.Error("GetUserRepoPermission[%d]: %v", baseRepo.ID, err)
		baseRepoPerm.AccessMode = perm.AccessModeNone
	}

	apiRepo := ToRepo(ctx, baseRepo, baseRepoPerm)
	baseBranchCache := make(map[string]*git_model.Branch)
	apiPullRequests := make([]*api.PullRequest, 0, len(prs))
	for _, pr := range prs {
		apiIssue := ToAPIIssue(ctx, doer, pr.Issue)

		apiPullRequest := &api.PullRequest{
			ID:             pr.ID,
			URL:            pr.Issue.HTMLURL(ctx),
			Index:          pr.Index,
			Poster:         apiIssue.Poster,
			Title:          apiIssue.Title,
			Body:           apiIssue.Body,
			Labels:         apiIssue.Labels,
			Milestone:      apiIssue.Milestone,
			Assignee:       apiIssue.Assignee,
			Assignees:      apiIssue.Assignees,
			State:          apiIssue.State,
			Draft:          pr.IsWorkInProgress(ctx),
			IsLocked:       apiIssue.IsLocked,
			Comments:       apiIssue.Comments,
			ReviewComments: reviewCounts[pr.IssueID],
			HTMLURL:        pr.Issue.HTMLURL(ctx),
			DiffURL:        pr.Issue.DiffURL(),
			PatchURL:       pr.Issue.PatchURL(),
			HasMerged:      pr.HasMerged,
			MergeBase:      pr.MergeBase,
			Mergeable:      pr.Mergeable(ctx),
			Deadline:       apiIssue.Deadline,
			Created:        pr.Issue.CreatedUnix.AsTimePtr(),
			Updated:        pr.Issue.UpdatedUnix.AsTimePtr(),
			PinOrder:       util.Iif(apiIssue.PinOrder == -1, 0, apiIssue.PinOrder),

			AllowMaintainerEdit: pr.AllowMaintainerEdit,

			Base: &api.PRBranchInfo{
				Name:       pr.BaseBranch,
				Ref:        pr.BaseBranch,
				RepoID:     pr.BaseRepoID,
				Repository: apiRepo,
			},
			Head: &api.PRBranchInfo{
				Name:   pr.HeadBranch,
				Ref:    pr.GetGitHeadRefName(),
				RepoID: -1,
			},
		}

		pr.RequestedReviewers = reviewersMap[pr.IssueID]
		for _, reviewer := range pr.RequestedReviewers {
			apiPullRequest.RequestedReviewers = append(apiPullRequest.RequestedReviewers, ToUser(ctx, reviewer, nil))
		}

		for _, reviewerTeam := range pr.RequestedReviewersTeams {
			convertedTeam, err := ToTeam(ctx, reviewerTeam, true)
			if err != nil {
				log.Error("LoadRequestedReviewersTeams[%d]: %v", pr.ID, err)
				return nil, err
			}

			apiPullRequest.RequestedReviewersTeams = append(apiPullRequest.RequestedReviewersTeams, convertedTeam)
		}

		if pr.Issue.ClosedUnix != 0 {
			apiPullRequest.Closed = pr.Issue.ClosedUnix.AsTimePtr()
		}

		baseBranch, ok := baseBranchCache[pr.BaseBranch]
		if !ok {
			baseBranch, err = git_model.GetBranch(ctx, baseRepo.ID, pr.BaseBranch)
			if err == nil {
				baseBranchCache[pr.BaseBranch] = baseBranch
			} else if !git_model.IsErrBranchNotExist(err) {
				return nil, err
			}
		}
		if baseBranch != nil {
			apiPullRequest.Base.Sha = baseBranch.CommitID
		}
		if pr.HeadRepoID == pr.BaseRepoID {
			apiPullRequest.Head.Repository = apiPullRequest.Base.Repository
		}

		// pull request head branch, both repository and branch could not exist
		if pr.HeadRepo != nil {
			apiPullRequest.Head.RepoID = pr.HeadRepo.ID
			exist, err := git_model.IsBranchExist(ctx, pr.HeadRepo.ID, pr.HeadBranch)
			if err != nil {
				log.Error("IsBranchExist[%d]: %v", pr.HeadRepo.ID, err)
				return nil, err
			}
			if exist {
				apiPullRequest.Head.Ref = pr.HeadBranch
			}
			if pr.HeadRepoID != pr.BaseRepoID {
				p, err := access_model.GetUserRepoPermission(ctx, pr.HeadRepo, doer)
				if err != nil {
					log.Error("GetUserRepoPermission[%d]: %v", pr.HeadRepoID, err)
					p.AccessMode = perm.AccessModeNone
				}
				apiPullRequest.Head.Repository = ToRepo(ctx, pr.HeadRepo, p)
			}
		}
		if apiPullRequest.Head.Ref == "" {
			apiPullRequest.Head.Ref = pr.GetGitHeadRefName()
		}

		if pr.Flow == issues_model.PullRequestFlowAGit {
			apiPullRequest.Head.Name = ""
		}
		apiPullRequest.Head.Sha, err = gitrepo.GetFullCommitID(ctx, baseRepo, pr.GetGitHeadRefName())
		if err != nil {
			log.Error("GetRefCommitID[%s]: %v", pr.GetGitHeadRefName(), err)
		}

		if len(apiPullRequest.Head.Sha) == 0 && len(apiPullRequest.Head.Ref) != 0 {
			headSHA, err := gitrepo.GetFullCommitID(ctx, baseRepo, apiPullRequest.Head.Ref)
			if err != nil && !git.IsErrNotExist(err) {
				log.Error("GetRefCommitID[%s]: %v", apiPullRequest.Head.Ref, err)
				return nil, err
			}
			if err == nil {
				apiPullRequest.Head.Sha = headSHA
			}
		}

		if pr.HasMerged {
			apiPullRequest.Merged = pr.MergedUnix.AsTimePtr()
			apiPullRequest.MergedCommitID = &pr.MergedCommitID
			apiPullRequest.MergedBy = ToUser(ctx, pr.Merger, nil)
		}

		// Do not provide "ChangeFiles/Additions/Deletions" for the PR list, because the "diff" is quite slow
		// If callers are interested in these values, they should do a separate request to get the PR details
		if apiPullRequest.ChangedFiles != nil || apiPullRequest.Additions != nil || apiPullRequest.Deletions != nil {
			setting.PanicInDevOrTesting("ChangedFiles/Additions/Deletions should not be set in PR list")
		}

		apiPullRequests = append(apiPullRequests, apiPullRequest)
	}

	return apiPullRequests, nil
}
