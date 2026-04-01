// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"strings"

	access_model "github.com/gitjet-ru/core-scm/models/perm/access"
	"github.com/gitjet-ru/core-scm/models/unit"
	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/gitjet-ru/core-scm/modules/gitrepo"
	api "github.com/gitjet-ru/core-scm/modules/structs"
	"github.com/gitjet-ru/core-scm/modules/util"
	"github.com/gitjet-ru/core-scm/routers/common"
	"github.com/gitjet-ru/core-scm/services/context"
)

// CompareDiff compare two branches or commits
func CompareDiff(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/compare/{basehead} repository repoCompareDiff
	// ---
	// summary: Get commit comparison information
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: basehead
	//   in: path
	//   description: compare two branches or commits
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Compare"
	//   "404":
	//     "$ref": "#/responses/notFound"

	_ = compareDiffRemote(ctx, ctx.PathParam("*"))
}

func compareDiffRemote(ctx *context.APIContext, compareParam string) bool {
	baseRepo := ctx.Repo.Repository
	compareReq := common.ParseCompareRouterParam(compareParam)
	if compareReq.BaseOriRefSuffix != "" {
		ctx.APIError(http.StatusBadRequest, "Unsupported comparison syntax: ref with suffix")
		return true
	}

	_, headRepo, err := common.GetHeadOwnerAndRepo(ctx, baseRepo, compareReq)
	switch {
	case errors.Is(err, util.ErrInvalidArgument):
		ctx.APIError(http.StatusBadRequest, err.Error())
		return true
	case errors.Is(err, util.ErrNotExist):
		ctx.APIErrorNotFound()
		return true
	case err != nil:
		ctx.APIErrorInternal(err)
		return true
	}

	permBase, err := access_model.GetUserRepoPermission(ctx, baseRepo, ctx.Doer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return true
	}
	if !permBase.CanRead(unit.TypeCode) {
		ctx.APIErrorNotFound("can't read baseRepo UnitTypeCode")
		return true
	}

	baseRef := util.IfZero(compareReq.BaseOriRef, baseRepo.GetPullRequestTargetBranch(ctx))
	headRef := util.IfZero(compareReq.HeadOriRef, headRepo.DefaultBranch)
	if strings.TrimSpace(baseRef) == "" || strings.TrimSpace(headRef) == "" {
		ctx.APIErrorNotFound()
		return true
	}

	baseCommitID, err := gitrepo.GetFullCommitID(ctx, baseRepo, baseRef)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return true
	}
	headCommitID, err := gitrepo.GetFullCommitID(ctx, headRepo, headRef)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return true
	}

	mergeBase := baseCommitID
	if !compareReq.DirectComparison() {
		if baseRepo.ID != headRepo.ID {
			if err := gitrepo.FetchRemoteCommit(ctx, headRepo, baseRepo, baseCommitID); err != nil {
				ctx.APIErrorInternal(err)
				return true
			}
		}
		mergeBase, err = gitrepo.MergeBase(ctx, headRepo, baseCommitID, headCommitID)
		if err != nil {
			ctx.APIErrorInternal(err)
			return true
		}
	}

	commitsInfo, err := gitrepo.RemoteListCommitsForAPI(ctx, headRepo, headCommitID, 0, 0, "", "", "", mergeBase)
	if err != nil {
		ctx.APIErrorInternal(err)
		return true
	}
	apiCommits := make([]*api.Commit, 0, len(commitsInfo))
	for _, c := range commitsInfo {
		apiCommits = append(apiCommits, commitInfoToAPICommit(ctx, c))
	}

	ctx.JSON(http.StatusOK, &api.Compare{
		TotalCommits: len(apiCommits),
		Commits:      apiCommits,
	})
	return true
}
