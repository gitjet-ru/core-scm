// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gitjet-ru/core-scm/modules/gitrepo"
	api "github.com/gitjet-ru/core-scm/modules/structs"
	"github.com/gitjet-ru/core-scm/modules/util"
	"github.com/gitjet-ru/core-scm/routers/api/v1/utils"
	"github.com/gitjet-ru/core-scm/services/context"
)

// GetGitAllRefs get ref or an list all the refs of a repository
func GetGitAllRefs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/refs repository repoListAllGitRefs
	// ---
	// summary: Get specified ref or filtered repository's refs
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
	// responses:
	//   "200":
	// #   "$ref": "#/responses/Reference" TODO: swagger doesn't support different output formats by ref
	//     "$ref": "#/responses/ReferenceList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	getGitRefsInternal(ctx, "")
}

// GetGitRefs get ref or an filteresd list of refs of a repository
func GetGitRefs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/refs/{ref} repository repoListGitRefs
	// ---
	// summary: Get specified ref or filtered repository's refs
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
	// - name: ref
	//   in: path
	//   description: part or full name of the ref
	//   type: string
	//   required: true
	// responses:
	//   "200":
	// #   "$ref": "#/responses/Reference" TODO: swagger doesn't support different output formats by ref
	//     "$ref": "#/responses/ReferenceList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	getGitRefsInternal(ctx, ctx.PathParam("*"))
}

func getGitRefsInternal(ctx *context.APIContext, filter string) {
	normalizedFilter := strings.TrimPrefix(filter, "/")
	if normalizedFilter != "" && !strings.HasPrefix(normalizedFilter, "refs/") {
		normalizedFilter = "refs/" + normalizedFilter
	}

	if gitrepo.UseRemoteReadBackendForAPI() {
		refs, err := gitrepo.RemoteListRefsForAPI(ctx, ctx.Repo.Repository, "refs/")
		if err != nil {
			ctx.APIErrorInternal(fmt.Errorf("RemoteListRefsForAPI: %w", err))
			return
		}
		apiRefs := make([]*api.Reference, 0, len(refs))
		for _, r := range refs {
			if normalizedFilter != "" && !strings.HasPrefix(r.GetName(), normalizedFilter) && r.GetName() != normalizedFilter {
				continue
			}
			refType := "commit"
			if strings.HasPrefix(r.GetName(), "refs/tags/") {
				refType = "tag"
			}
			apiRefs = append(apiRefs, &api.Reference{
				Ref: r.GetName(),
				URL: ctx.Repo.Repository.APIURL() + "/git/" + util.PathEscapeSegments(r.GetName()),
				Object: &api.GitObject{
					SHA:  r.GetObjectId(),
					Type: refType,
					URL:  ctx.Repo.Repository.APIURL() + "/git/" + url.PathEscape(refType) + "s/" + url.PathEscape(r.GetObjectId()),
				},
			})
		}
		if len(apiRefs) == 0 {
			ctx.APIErrorNotFound()
			return
		}
		if len(apiRefs) == 1 && apiRefs[0].Ref == normalizedFilter {
			ctx.JSON(http.StatusOK, &apiRefs[0])
			return
		}
		ctx.JSON(http.StatusOK, &apiRefs)
		return
	}

	refs, lastMethodName, err := utils.GetGitRefs(ctx, normalizedFilter)
	if err != nil {
		ctx.APIErrorInternal(fmt.Errorf("%s: %w", lastMethodName, err))
		return
	}

	if len(refs) == 0 {
		ctx.APIErrorNotFound()
		return
	}

	apiRefs := make([]*api.Reference, len(refs))
	for i := range refs {
		apiRefs[i] = &api.Reference{
			Ref: refs[i].Name,
			URL: ctx.Repo.Repository.APIURL() + "/git/" + util.PathEscapeSegments(refs[i].Name),
			Object: &api.GitObject{
				SHA:  refs[i].Object.String(),
				Type: refs[i].Type,
				URL:  ctx.Repo.Repository.APIURL() + "/git/" + url.PathEscape(refs[i].Type) + "s/" + url.PathEscape(refs[i].Object.String()),
			},
		}
	}
	// If single reference is found and it matches filter exactly return it as object
	if len(apiRefs) == 1 && apiRefs[0].Ref == normalizedFilter {
		ctx.JSON(http.StatusOK, &apiRefs[0])
		return
	}
	ctx.JSON(http.StatusOK, &apiRefs)
}
