// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"errors"

	git_model "github.com/gitjet-ru/core-scm/models/git"
	repo_model "github.com/gitjet-ru/core-scm/models/repo"
	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/gitjet-ru/core-scm/modules/gitrepo"
	"github.com/gitjet-ru/core-scm/modules/reqctx"
	"github.com/gitjet-ru/core-scm/services/context"
)

type RefCommit struct {
	InputRef string
	RefName  git.RefName
	Commit   *git.Commit
	CommitID string
}

// ResolveRefCommit resolve ref to a commit if exist
func ResolveRefCommit(ctx reqctx.RequestContext, repo *repo_model.Repository, inputRef string, minCommitIDLen ...int) (_ *RefCommit, err error) {
	refCommit := RefCommit{InputRef: inputRef}
	if exist, _ := git_model.IsBranchExist(ctx, repo.ID, inputRef); exist {
		refCommit.RefName = git.RefNameFromBranch(inputRef)
	} else if gitrepo.IsTagExist(ctx, repo, inputRef) {
		refCommit.RefName = git.RefNameFromTag(inputRef)
	} else if git.IsStringLikelyCommitID(git.ObjectFormatFromName(repo.ObjectFormatName), inputRef, minCommitIDLen...) {
		refCommit.RefName = git.RefNameFromCommit(inputRef)
	}
	if refCommit.RefName == "" {
		return nil, git.ErrNotExist{ID: inputRef}
	}
	refCommit.CommitID, err = gitrepo.GetFullCommitID(ctx, repo, refCommit.RefName.String())
	if err != nil {
		return nil, err
	}
	if apiCtx, ok := ctx.(*context.APIContext); ok && apiCtx.Repo != nil && apiCtx.Repo.GitRepo != nil {
		refCommit.Commit, _ = apiCtx.Repo.GitRepo.GetCommit(refCommit.RefName.String())
	}
	return &refCommit, nil
}

func NewRefCommit(refName git.RefName, commit *git.Commit) *RefCommit {
	return &RefCommit{InputRef: refName.ShortName(), RefName: refName, Commit: commit, CommitID: commit.ID.String()}
}

// GetGitRefs return git references based on filter
func GetGitRefs(ctx *context.APIContext, filter string) ([]*git.Reference, string, error) {
	if ctx.Repo.GitRepo == nil {
		return nil, "", errors.New("no open git repo found in context")
	}
	if len(filter) > 0 {
		filter = "refs/" + filter
	}
	refs, err := ctx.Repo.GitRepo.GetRefsFiltered(filter)
	return refs, "GetRefsFiltered", err
}
