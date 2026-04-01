// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	gitstoragev1 "github.com/gitjet-ru/git-storage/gen/go/gitstorage/v1"
	repo_model "github.com/gitjet-ru/core-scm/models/repo"
	"github.com/gitjet-ru/core-scm/modules/gitrepo"
	"github.com/gitjet-ru/core-scm/modules/setting"
	api "github.com/gitjet-ru/core-scm/modules/structs"
	"github.com/gitjet-ru/core-scm/modules/util"
	"github.com/gitjet-ru/core-scm/modules/web"
	"github.com/gitjet-ru/core-scm/services/context"
	notify_service "github.com/gitjet-ru/core-scm/services/notify"
	wiki_service "github.com/gitjet-ru/core-scm/services/wiki"
)

// NewWikiPage response for wiki create request
func NewWikiPage(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/wiki/new repository repoCreateWikiPage
	// ---
	// summary: Create a wiki page
	// consumes:
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateWikiPageOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/WikiPage"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := web.GetForm(ctx).(*api.CreateWikiPageOptions)

	if util.IsEmptyString(form.Title) {
		ctx.APIError(http.StatusBadRequest, nil)
		return
	}

	wikiName := wiki_service.UserTitleToWebPath("", form.Title)

	if len(form.Message) == 0 {
		form.Message = fmt.Sprintf("Add %q", form.Title)
	}

	content, err := base64.StdEncoding.DecodeString(form.ContentBase64)
	if err != nil {
		ctx.APIError(http.StatusBadRequest, err)
		return
	}
	form.ContentBase64 = string(content)

	if err := wiki_service.AddWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, wikiName, form.ContentBase64, form.Message); err != nil {
		if repo_model.IsErrWikiReservedName(err) {
			ctx.APIError(http.StatusBadRequest, err)
		} else if repo_model.IsErrWikiAlreadyExist(err) {
			ctx.APIError(http.StatusBadRequest, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	wikiPage := getWikiPage(ctx, wikiName)

	if !ctx.Written() {
		notify_service.NewWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(wikiName), form.Message)
		ctx.JSON(http.StatusCreated, wikiPage)
	}
}

// EditWikiPage response for wiki modify request
func EditWikiPage(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/wiki/page/{pageName} repository repoEditWikiPage
	// ---
	// summary: Edit a wiki page
	// consumes:
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
	// - name: pageName
	//   in: path
	//   description: name of the page
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateWikiPageOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/WikiPage"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := web.GetForm(ctx).(*api.CreateWikiPageOptions)

	oldWikiName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("pageName"))
	newWikiName := wiki_service.UserTitleToWebPath("", form.Title)

	if len(newWikiName) == 0 {
		newWikiName = oldWikiName
	}

	if len(form.Message) == 0 {
		form.Message = fmt.Sprintf("Update %q", newWikiName)
	}

	content, err := base64.StdEncoding.DecodeString(form.ContentBase64)
	if err != nil {
		ctx.APIError(http.StatusBadRequest, err)
		return
	}
	form.ContentBase64 = string(content)

	if err := wiki_service.EditWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, oldWikiName, newWikiName, form.ContentBase64, form.Message); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	wikiPage := getWikiPage(ctx, newWikiName)

	if !ctx.Written() {
		notify_service.EditWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(newWikiName), form.Message)
		ctx.JSON(http.StatusOK, wikiPage)
	}
}

func getWikiPage(ctx *context.APIContext, wikiName wiki_service.WebPath) *api.WikiPage {
	return getWikiPageRemote(ctx, wikiName)
}

func getWikiPageRemote(ctx *context.APIContext, wikiName wiki_service.WebPath) *api.WikiPage {
	wikiRepo := ctx.Repo.Repository.WikiStorageRepo()
	branch := ctx.Repo.Repository.DefaultWikiBranch

	content, pageFilename, found, err := getWikiFileContentRemote(ctx, wikiRepo, branch, wikiName)
	if err != nil {
		ctx.APIErrorInternal(err)
		return nil
	}
	if !found {
		ctx.APIErrorNotFound()
		return nil
	}

	sidebar, _, _, err := getWikiFileContentRemote(ctx, wikiRepo, branch, "_Sidebar")
	if err != nil {
		ctx.APIErrorInternal(err)
		return nil
	}
	footer, _, _, err := getWikiFileContentRemote(ctx, wikiRepo, branch, "_Footer")
	if err != nil {
		ctx.APIErrorInternal(err)
		return nil
	}

	commitsCount, _ := gitrepo.FileCommitsCount(ctx, wikiRepo, branch, pageFilename)
	lastCommits, err := gitrepo.RemoteListCommitsForAPI(ctx, wikiRepo, branch, 1, 0, pageFilename, "", "", "")
	if err != nil {
		ctx.APIErrorInternal(err)
		return nil
	}
	if len(lastCommits) == 0 {
		ctx.APIErrorNotFound()
		return nil
	}
	lastCommit := lastCommits[0]
	_, title := wiki_service.WebPathToUserTitle(wikiName)

	return &api.WikiPage{
		WikiPageMetaData: &api.WikiPageMetaData{
			Title:   title,
			HTMLURL: ctx.Repo.Repository.HTMLURL() + "/wiki/" + string(wikiName),
			SubURL:  string(wikiName),
			LastCommit: &api.WikiCommit{
				ID: lastCommit.GetId(),
				Author: &api.CommitUser{
					Identity: api.Identity{Name: lastCommit.GetAuthorName(), Email: lastCommit.GetAuthorEmail()},
				},
				Committer: &api.CommitUser{
					Identity: api.Identity{Name: lastCommit.GetCommitterName(), Email: lastCommit.GetCommitterEmail()},
				},
				Message: strings.TrimSpace(lastCommit.GetSubject() + "\n\n" + lastCommit.GetBody()),
			},
		},
		ContentBase64: content,
		CommitCount:   commitsCount,
		Sidebar:       sidebar,
		Footer:        footer,
	}
}

func getWikiFileContentRemote(ctx *context.APIContext, repo repo_model.StorageRepo, branch string, wikiName wiki_service.WebPath) (contentBase64, filename string, found bool, err error) {
	maxBlobSize := int32(setting.API.DefaultMaxBlobSize)
	if setting.API.DefaultMaxBlobSize > int64(^uint32(0)>>1) {
		maxBlobSize = int32(^uint32(0) >> 1)
	}
	gitFilename := wiki_service.WebPathToGitPath(wikiName)
	paths := []string{gitFilename}
	if unescaped, ueErr := url.QueryUnescape(gitFilename); ueErr == nil && unescaped != "" && unescaped != gitFilename {
		paths = append(paths, unescaped)
	}
	for _, candidate := range paths {
		blobResp, blobErr := gitrepo.RemoteGetBlobForAPI(ctx, repo, branch, candidate, maxBlobSize)
		if blobErr != nil {
			continue
		}
		if blobResp.GetTruncated() {
			return "", candidate, true, nil
		}
		return base64.StdEncoding.EncodeToString(blobResp.GetContent()), candidate, true, nil
	}
	return "", "", false, nil
}

func commitInfoToWikiCommit(c *gitstoragev1.CommitInfo) *api.WikiCommit {
	return &api.WikiCommit{
		ID: c.GetId(),
		Author: &api.CommitUser{
			Identity: api.Identity{Name: c.GetAuthorName(), Email: c.GetAuthorEmail()},
			Date:     time.Unix(c.GetAuthorUnix(), 0).UTC().Format(time.RFC3339),
		},
		Committer: &api.CommitUser{
			Identity: api.Identity{Name: c.GetCommitterName(), Email: c.GetCommitterEmail()},
			Date:     time.Unix(c.GetCommitterUnix(), 0).UTC().Format(time.RFC3339),
		},
		Message: strings.TrimSpace(c.GetSubject() + "\n\n" + c.GetBody()),
	}
}

// DeleteWikiPage delete wiki page
func DeleteWikiPage(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/wiki/page/{pageName} repository repoDeleteWikiPage
	// ---
	// summary: Delete a wiki page
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
	// - name: pageName
	//   in: path
	//   description: name of the page
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	wikiName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("pageName"))

	if err := wiki_service.DeleteWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, wikiName); err != nil {
		if err.Error() == "file does not exist" {
			ctx.APIErrorNotFound(err)
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	notify_service.DeleteWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(wikiName))

	ctx.Status(http.StatusNoContent)
}

// ListWikiPages get wiki pages list
func ListWikiPages(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/wiki/pages repository repoGetWikiPages
	// ---
	// summary: Get all wiki pages
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/WikiPageList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	page := max(ctx.FormInt("page"), 1)
	limit := ctx.FormInt("limit")
	if limit <= 1 {
		limit = setting.API.DefaultPagingNum
	}

	skip := (page - 1) * limit
	maxNum := page * limit

	repo := ctx.Repo.Repository.WikiStorageRepo()
	branch := ctx.Repo.Repository.DefaultWikiBranch
	entries, _, err := gitrepo.RemoteGetTreeForAPI(ctx, repo, branch, "", false, 5000)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	pages := make([]*api.WikiPageMetaData, 0, len(entries))
	for i, entry := range entries {
		if i < skip || i >= maxNum || !strings.EqualFold(entry.GetObjectType(), "blob") {
			continue
		}
		entryPath := entry.GetPath()
		commits, err := gitrepo.RemoteListCommitsForAPI(ctx, repo, branch, 1, 0, entryPath, "", "", "")
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		if len(commits) == 0 {
			continue
		}
		wikiName, err := wiki_service.GitPathToWebPath(entryPath)
		if err != nil {
			if repo_model.IsErrWikiInvalidFileName(err) {
				continue
			}
			ctx.APIErrorInternal(err)
			return
		}
		_, title := wiki_service.WebPathToUserTitle(wikiName)
		pages = append(pages, &api.WikiPageMetaData{
			Title:      title,
			HTMLURL:    ctx.Repo.Repository.HTMLURL() + "/wiki/" + string(wikiName),
			SubURL:     string(wikiName),
			LastCommit: commitInfoToWikiCommit(commits[0]),
		})
	}

	ctx.SetLinkHeader(int64(len(entries)), limit)
	ctx.SetTotalCountHeader(int64(len(entries)))
	ctx.JSON(http.StatusOK, pages)
}

// GetWikiPage get single wiki page
func GetWikiPage(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/wiki/page/{pageName} repository repoGetWikiPage
	// ---
	// summary: Get a wiki page
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
	// - name: pageName
	//   in: path
	//   description: name of the page
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/WikiPage"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// get requested pagename
	pageName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("pageName"))

	wikiPage := getWikiPage(ctx, pageName)
	if !ctx.Written() {
		ctx.JSON(http.StatusOK, wikiPage)
	}
}

// ListPageRevisions renders file revision list of wiki page
func ListPageRevisions(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/wiki/revisions/{pageName} repository repoGetWikiPageRevisions
	// ---
	// summary: Get revisions of a wiki page
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
	// - name: pageName
	//   in: path
	//   description: name of the page
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/WikiCommitList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// get requested pagename
	pageName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("pageName"))
	if len(pageName) == 0 {
		pageName = "Home"
	}

	repo := ctx.Repo.Repository.WikiStorageRepo()
	branch := ctx.Repo.Repository.DefaultWikiBranch
	_, pageFilename, found, err := getWikiFileContentRemote(ctx, repo, branch, pageName)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if !found {
		ctx.APIErrorNotFound()
		return
	}
	commitsCount, _ := gitrepo.FileCommitsCount(ctx, repo, branch, pageFilename)
	page := max(ctx.FormInt("page"), 1)
	limit := setting.API.DefaultPagingNum
	skip := (page - 1) * limit
	commitsInfo, err := gitrepo.RemoteListCommitsForAPI(ctx, repo, branch, int32(limit), int32(skip), pageFilename, "", "", "")
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	result := make([]*api.WikiCommit, 0, len(commitsInfo))
	for _, c := range commitsInfo {
		result = append(result, commitInfoToWikiCommit(c))
	}

	// FIXME: SetLinkHeader missing
	ctx.SetTotalCountHeader(commitsCount)
	ctx.JSON(http.StatusOK, &api.WikiCommitList{
		WikiCommits: result,
		Count:       commitsCount,
	})
}

