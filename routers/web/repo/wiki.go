// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"html/template"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/gitjet-ru/core-scm/models/renderhelper"
	repo_model "github.com/gitjet-ru/core-scm/models/repo"
	"github.com/gitjet-ru/core-scm/models/unit"
	"github.com/gitjet-ru/core-scm/modules/base"
	"github.com/gitjet-ru/core-scm/modules/charset"
	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/gitjet-ru/core-scm/modules/gitrepo"
	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/markup"
	"github.com/gitjet-ru/core-scm/modules/markup/markdown"
	"github.com/gitjet-ru/core-scm/modules/setting"
	api "github.com/gitjet-ru/core-scm/modules/structs"
	"github.com/gitjet-ru/core-scm/modules/templates"
	"github.com/gitjet-ru/core-scm/modules/timeutil"
	"github.com/gitjet-ru/core-scm/modules/util"
	"github.com/gitjet-ru/core-scm/modules/web"
	"github.com/gitjet-ru/core-scm/services/context"
	"github.com/gitjet-ru/core-scm/services/forms"
	notify_service "github.com/gitjet-ru/core-scm/services/notify"
	repo_service "github.com/gitjet-ru/core-scm/services/repository"
	wiki_service "github.com/gitjet-ru/core-scm/services/wiki"
)

const (
	tplWikiStart    templates.TplName = "repo/wiki/start"
	tplWikiView     templates.TplName = "repo/wiki/view"
	tplWikiRevision templates.TplName = "repo/wiki/revision"
	tplWikiNew      templates.TplName = "repo/wiki/new"
	tplWikiPages    templates.TplName = "repo/wiki/pages"
)

// MustEnableWiki check if wiki is enabled, if external then redirect
func MustEnableWiki(ctx *context.Context) {
	if !ctx.Repo.CanRead(unit.TypeWiki) &&
		!ctx.Repo.CanRead(unit.TypeExternalWiki) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User %-v cannot read %-v or %-v of repo %-v\n"+
				"User in repo has Permissions: %-+v",
				ctx.Doer,
				unit.TypeWiki,
				unit.TypeExternalWiki,
				ctx.Repo.Repository,
				ctx.Repo.Permission)
		}
		ctx.NotFound(nil)
		return
	}

	repoUnit, err := ctx.Repo.Repository.GetUnit(ctx, unit.TypeExternalWiki)
	if err == nil {
		ctx.Redirect(repoUnit.ExternalWikiConfig().ExternalWikiURL)
		return
	}
}

// PageMeta wiki page meta information
type PageMeta struct {
	Name         string
	SubURL       string
	GitEntryName string
	UpdatedUnix  timeutil.TimeStamp
}


func renderViewPage(ctx *context.Context) (string, bool) {
	repo := ctx.Repo.Repository.WikiStorageRepo()
	branch := ctx.Repo.Repository.DefaultWikiBranch
	treeEntries, _, err := gitrepo.RemoteGetTreeForAPI(ctx, repo, branch, "", false, 5000)
	if err != nil {
		ctx.ServerError("RemoteGetTreeForAPI", err)
		return "", false
	}
	pages := make([]PageMeta, 0, len(treeEntries))
	for _, entry := range treeEntries {
		if !strings.EqualFold(entry.GetObjectType(), "blob") {
			continue
		}
		entryPath := entry.GetPath()
		wikiName, err := wiki_service.GitPathToWebPath(entryPath)
		if err != nil {
			if repo_model.IsErrWikiInvalidFileName(err) {
				continue
			}
			ctx.ServerError("WikiFilenameToName", err)
			return "", false
		} else if wikiName == "_Sidebar" || wikiName == "_Footer" {
			continue
		}
		_, displayName := wiki_service.WebPathToUserTitle(wikiName)
		pages = append(pages, PageMeta{
			Name:         displayName,
			SubURL:       wiki_service.WebPathToURLPath(wikiName),
			GitEntryName: entryPath,
		})
	}
	ctx.Data["Pages"] = pages

	// get requested page name
	pageName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if len(pageName) == 0 {
		pageName = "Home"
	}

	_, displayName := wiki_service.WebPathToUserTitle(pageName)
	ctx.Data["PageURL"] = wiki_service.WebPathToURLPath(pageName)
	ctx.Data["old_title"] = displayName
	ctx.Data["Title"] = displayName
	ctx.Data["title"] = displayName

	isSideBar := pageName == "_Sidebar"
	isFooter := pageName == "_Footer"

	// lookup filename in wiki - get gitTree entry , real filename
	data, pageFilename, found, isRaw, err := getWikiContentForEditRemote(ctx, pageName)
	if !found {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/?action=_pages")
	}
	if isRaw {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/raw/" + string(pageName))
	}
	if err != nil || ctx.Written() {
		if err != nil {
			ctx.ServerError("getWikiContentForEditRemote", err)
		}
		return "", false
	}

	rctx := renderhelper.NewRenderContextRepoWiki(ctx, ctx.Repo.Repository)

	renderFn := func(data []byte) (escaped *charset.EscapeStatus, output template.HTML, err error) {
		buf := &strings.Builder{}
		markupRd, markupWr := io.Pipe()
		defer markupWr.Close()
		done := make(chan struct{})
		go func() {
			// We allow NBSP here this is rendered
			escaped, _ = charset.EscapeControlReader(markupRd, buf, ctx.Locale, charset.RuneNBSP)
			output = template.HTML(buf.String())
			buf.Reset()
			close(done)
		}()

		err = markdown.Render(rctx, bytes.NewReader(data), markupWr)
		_ = markupWr.CloseWithError(err)
		<-done
		return escaped, output, err
	}

	ctx.Data["EscapeStatus"], ctx.Data["WikiContentHTML"], err = renderFn(data)
	if err != nil {
		ctx.ServerError("Render", err)
		return "", false
	}

	if rctx.TocShowInSection == markup.TocShowInSidebar && len(rctx.TocHeadingItems) > 0 {
		sb := strings.Builder{}
		markup.RenderTocHeadingItems(rctx, map[string]string{"open": ""}, &sb)
		ctx.Data["WikiSidebarTocHTML"] = template.HTML(sb.String())
	}

	if !isSideBar {
		sidebarContent := []byte{}
		if sData, _, sFound, _, sErr := getWikiContentForEditRemote(ctx, "_Sidebar"); sErr == nil && sFound {
			sidebarContent = sData
		}
		if ctx.Written() {
			return "", false
		}
		ctx.Data["WikiSidebarEscapeStatus"], ctx.Data["WikiSidebarHTML"], err = renderFn(sidebarContent)
		if err != nil {
			ctx.ServerError("Render", err)
			return "", false
		}
	}

	if !isFooter {
		footerContent := []byte{}
		if fData, _, fFound, _, fErr := getWikiContentForEditRemote(ctx, "_Footer"); fErr == nil && fFound {
			footerContent = fData
		}
		if ctx.Written() {
			return "", false
		}
		ctx.Data["WikiFooterEscapeStatus"], ctx.Data["WikiFooterHTML"], err = renderFn(footerContent)
		if err != nil {
			ctx.ServerError("Render", err)
			return "", false
		}
	}

	// get commit count - wiki revisions
	commitsCount, _ := gitrepo.FileCommitsCount(ctx, ctx.Repo.Repository.WikiStorageRepo(), ctx.Repo.Repository.DefaultWikiBranch, pageFilename)
	ctx.Data["CommitCount"] = commitsCount
	if commits, cErr := gitrepo.RemoteListCommitsForAPI(ctx, repo, branch, 1, 0, pageFilename, "", "", ""); cErr == nil && len(commits) > 0 {
		ctx.Data["Author"] = &git.Signature{
			Name:  commits[0].GetAuthorName(),
			Email: commits[0].GetAuthorEmail(),
			When:  timeutil.TimeStamp(commits[0].GetAuthorUnix()).AsTime(),
		}
	}

	return pageFilename, true
}

func renderRevisionPage(ctx *context.Context) (string, bool) {
	// get requested page name
	pageName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if len(pageName) == 0 {
		pageName = "Home"
	}

	_, displayName := wiki_service.WebPathToUserTitle(pageName)
	ctx.Data["PageURL"] = wiki_service.WebPathToURLPath(pageName)
	ctx.Data["old_title"] = displayName
	ctx.Data["Title"] = displayName
	ctx.Data["title"] = displayName

	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name

	_, pageFilename, found, _, err := getWikiContentForEditRemote(ctx, pageName)
	if err != nil {
		ctx.ServerError("getWikiContentForEditRemote", err)
		return "", false
	}
	if !found {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/?action=_pages")
	}
	if ctx.Written() {
		return "", false
	}

	// get commit count - wiki revisions
	commitsCount, _ := gitrepo.FileCommitsCount(ctx, ctx.Repo.Repository.WikiStorageRepo(), ctx.Repo.Repository.DefaultWikiBranch, pageFilename)
	ctx.Data["CommitCount"] = commitsCount

	// get page
	page := max(ctx.FormInt("page"), 1)

	limit := setting.Git.CommitsRangeSize
	skip := (page - 1) * limit
	commitsInfo, err := gitrepo.RemoteListCommitsForAPI(
		ctx,
		ctx.Repo.Repository.WikiStorageRepo(),
		ctx.Repo.Repository.DefaultWikiBranch,
		int32(limit),
		int32(skip),
		pageFilename,
		"",
		"",
		"",
	)
	if err != nil {
		ctx.ServerError("RemoteListCommitsForAPI", err)
		return "", false
	}
	wikiCommits := make([]*api.WikiCommit, 0, len(commitsInfo))
	for _, c := range commitsInfo {
		wikiCommits = append(wikiCommits, &api.WikiCommit{
			ID: c.GetId(),
			Author: &api.CommitUser{
				Identity: api.Identity{Name: c.GetAuthorName(), Email: c.GetAuthorEmail()},
				Date:     timeutil.TimeStamp(c.GetAuthorUnix()).AsTime().UTC().Format(time.RFC3339),
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{Name: c.GetCommitterName(), Email: c.GetCommitterEmail()},
				Date:     timeutil.TimeStamp(c.GetCommitterUnix()).AsTime().UTC().Format(time.RFC3339),
			},
			Message: strings.TrimSpace(c.GetSubject() + "\n\n" + c.GetBody()),
		})
	}
	ctx.Data["WikiCommits"] = wikiCommits
	if len(wikiCommits) > 0 {
		ctx.Data["Author"] = &git.Signature{
			Name:  wikiCommits[0].Author.Name,
			Email: wikiCommits[0].Author.Email,
			When:  timeutil.TimeStamp(commitsInfo[0].GetAuthorUnix()).AsTime(),
		}
	}

	pager := context.NewPagination(commitsCount, setting.Git.CommitsRangeSize, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	return pageFilename, true
}

func renderEditPage(ctx *context.Context) {
	// get requested page name
	pageName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if len(pageName) == 0 {
		pageName = "Home"
	}

	_, displayName := wiki_service.WebPathToUserTitle(pageName)
	ctx.Data["PageURL"] = wiki_service.WebPathToURLPath(pageName)
	ctx.Data["old_title"] = displayName
	ctx.Data["Title"] = displayName
	ctx.Data["title"] = displayName

	data, _, found, isRaw, err := getWikiContentForEditRemote(ctx, pageName)
	if err != nil {
		ctx.ServerError("getWikiContentForEditRemote", err)
		return
	}
	if !found {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/?action=_pages")
	}
	if isRaw {
		ctx.HTTPError(http.StatusForbidden, "Editing of raw wiki files is not allowed")
	}
	if ctx.Written() {
		return
	}

	ctx.Data["WikiEditContent"] = string(data)
}

func getWikiContentForEditRemote(ctx *context.Context, wikiName wiki_service.WebPath) (content []byte, filename string, found bool, isRaw bool, err error) {
	repo := ctx.Repo.Repository.WikiStorageRepo()
	branch := ctx.Repo.Repository.DefaultWikiBranch
	maxBytes := int32(5 * 1024 * 1024)

	primary := wiki_service.WebPathToGitPath(wikiName)
	blobResp, blobErr := gitrepo.RemoteGetBlobForAPI(ctx, repo, branch, primary, maxBytes)
	if blobErr == nil {
		return blobResp.GetContent(), primary, true, false, nil
	}

	secondary := strings.TrimSuffix(primary, ".md")
	blobResp, blobErr = gitrepo.RemoteGetBlobForAPI(ctx, repo, branch, secondary, maxBytes)
	if blobErr == nil {
		return blobResp.GetContent(), secondary, true, true, nil
	}
	return nil, "", false, false, nil
}

// WikiPost renders post of wiki page
func WikiPost(ctx *context.Context) {
	switch ctx.FormString("action") {
	case "_new":
		if !ctx.Repo.CanWrite(unit.TypeWiki) {
			ctx.NotFound(nil)
			return
		}
		NewWikiPost(ctx)
		return
	case "_delete":
		if !ctx.Repo.CanWrite(unit.TypeWiki) {
			ctx.NotFound(nil)
			return
		}
		DeleteWikiPagePost(ctx)
		return
	}

	if !ctx.Repo.CanWrite(unit.TypeWiki) {
		ctx.NotFound(nil)
		return
	}
	EditWikiPost(ctx)
}

// Wiki renders single wiki page
func Wiki(ctx *context.Context) {
	ctx.Data["CanWriteWiki"] = ctx.Repo.CanWrite(unit.TypeWiki) && !ctx.Repo.Repository.IsArchived

	switch ctx.FormString("action") {
	case "_pages":
		WikiPages(ctx)
		return
	case "_revision":
		WikiRevision(ctx)
		return
	case "_edit":
		if !ctx.Repo.CanWrite(unit.TypeWiki) {
			ctx.NotFound(nil)
			return
		}
		EditWiki(ctx)
		return
	case "_new":
		if !ctx.Repo.CanWrite(unit.TypeWiki) {
			ctx.NotFound(nil)
			return
		}
		NewWiki(ctx)
		return
	}

	if !repo_service.HasWiki(ctx, ctx.Repo.Repository) {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(http.StatusOK, tplWikiStart)
		return
	}

	wikiPath, ok := renderViewPage(ctx)
	if ctx.Written() {
		return
	}
	if !ok {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(http.StatusOK, tplWikiStart)
		return
	}

	detectedRender := markup.DetectRendererTypeByFilename(wikiPath)
	if detectedRender == nil || detectedRender.Name() != markdown.MarkupName {
		ctx.Data["FormatWarning"] = "File extension " + path.Ext(wikiPath) + " is not supported at the moment. Rendered as Markdown."
	}

	ctx.HTML(http.StatusOK, tplWikiView)
}

// WikiRevision renders file revision list of wiki page
func WikiRevision(ctx *context.Context) {
	ctx.Data["CanWriteWiki"] = ctx.Repo.CanWrite(unit.TypeWiki) && !ctx.Repo.Repository.IsArchived

	if !repo_service.HasWiki(ctx, ctx.Repo.Repository) {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(http.StatusOK, tplWikiStart)
		return
	}

	_, ok := renderRevisionPage(ctx)
	if ctx.Written() {
		return
	}
	if !ok {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(http.StatusOK, tplWikiStart)
		return
	}

	ctx.HTML(http.StatusOK, tplWikiRevision)
}

// WikiPages render wiki pages list page
func WikiPages(ctx *context.Context) {
	if !repo_service.HasWiki(ctx, ctx.Repo.Repository) {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki")
		return
	}

	ctx.Data["Title"] = ctx.Tr("repo.wiki.pages")
	ctx.Data["CanWriteWiki"] = ctx.Repo.CanWrite(unit.TypeWiki) && !ctx.Repo.Repository.IsArchived

	repo := ctx.Repo.Repository.WikiStorageRepo()
	branch := ctx.Repo.Repository.DefaultWikiBranch
	treeEntries, _, err := gitrepo.RemoteGetTreeForAPI(ctx, repo, branch, "", false, 5000)
	if err != nil {
		ctx.ServerError("RemoteGetTreeForAPI", err)
		return
	}
	sort.Slice(treeEntries, func(i, j int) bool {
		return base.NaturalSortCompare(treeEntries[i].GetPath(), treeEntries[j].GetPath()) < 0
	})

	pages := make([]PageMeta, 0, len(treeEntries))
	for _, entry := range treeEntries {
		if !strings.EqualFold(entry.GetObjectType(), "blob") {
			continue
		}
		entryPath := entry.GetPath()
		wikiName, err := wiki_service.GitPathToWebPath(entryPath)
		if err != nil {
			if repo_model.IsErrWikiInvalidFileName(err) {
				continue
			}
			ctx.ServerError("WikiFilenameToName", err)
			return
		}
		commits, err := gitrepo.RemoteListCommitsForAPI(ctx, repo, branch, 1, 0, entryPath, "", "", "")
		if err != nil {
			ctx.ServerError("RemoteListCommitsForAPI", err)
			return
		}
		var updatedUnix timeutil.TimeStamp
		if len(commits) > 0 {
			updatedUnix = timeutil.TimeStamp(commits[0].GetAuthorUnix())
		}
		_, displayName := wiki_service.WebPathToUserTitle(wikiName)
		pages = append(pages, PageMeta{
			Name:         displayName,
			SubURL:       wiki_service.WebPathToURLPath(wikiName),
			GitEntryName: entryPath,
			UpdatedUnix:  updatedUnix,
		})
	}
	ctx.Data["Pages"] = pages

	ctx.HTML(http.StatusOK, tplWikiPages)
}

// WikiRaw outputs raw blob requested by user (image for example)
func WikiRaw(ctx *context.Context) {
	providedWebPath := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	providedGitPath := wiki_service.WebPathToGitPath(providedWebPath)
	repo := ctx.Repo.Repository.WikiStorageRepo()
	branch := ctx.Repo.Repository.DefaultWikiBranch
	tryPaths := []string{providedGitPath, strings.TrimSuffix(providedGitPath, ".md")}
	for _, candidate := range tryPaths {
		blobResp, err := gitrepo.RemoteGetBlobForAPI(ctx, repo, branch, candidate, 0)
		if err != nil {
			continue
		}
		ctx.Resp.Header().Set("Content-Type", "application/octet-stream")
		if _, wErr := ctx.Resp.Write(blobResp.GetContent()); wErr != nil {
			ctx.ServerError("Write", wErr)
		}
		return
	}
	ctx.NotFound(nil)
}

// NewWiki render wiki create page
func NewWiki(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.wiki.new_page")

	if !repo_service.HasWiki(ctx, ctx.Repo.Repository) {
		ctx.Data["title"] = "Home"
	}
	if ctx.FormString("title") != "" {
		ctx.Data["title"] = ctx.FormString("title")
	}

	ctx.HTML(http.StatusOK, tplWikiNew)
}

// NewWikiPost response for wiki create request
func NewWikiPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewWikiForm)
	ctx.Data["Title"] = ctx.Tr("repo.wiki.new_page")

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplWikiNew)
		return
	}

	if util.IsEmptyString(form.Title) {
		ctx.RenderWithErrDeprecated(ctx.Tr("repo.issues.new.title_empty"), tplWikiNew, form)
		return
	}

	wikiName := wiki_service.UserTitleToWebPath("", form.Title)

	if len(form.Message) == 0 {
		form.Message = ctx.Locale.TrString("repo.editor.add", form.Title)
	}

	if err := wiki_service.AddWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, wikiName, form.Content, form.Message); err != nil {
		if repo_model.IsErrWikiReservedName(err) {
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("repo.wiki.reserved_page", wikiName), tplWikiNew, &form)
		} else if repo_model.IsErrWikiAlreadyExist(err) {
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("repo.wiki.page_already_exists"), tplWikiNew, &form)
		} else {
			ctx.ServerError("AddWikiPage", err)
		}
		return
	}

	notify_service.NewWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(wikiName), form.Message)

	ctx.Redirect(ctx.Repo.RepoLink + "/wiki/" + wiki_service.WebPathToURLPath(wikiName))
}

// EditWiki render wiki modify page
func EditWiki(ctx *context.Context) {
	ctx.Data["PageIsWikiEdit"] = true

	if !repo_service.HasWiki(ctx, ctx.Repo.Repository) {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki")
		return
	}

	renderEditPage(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplWikiNew)
}

// EditWikiPost response for wiki modify request
func EditWikiPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewWikiForm)
	ctx.Data["Title"] = ctx.Tr("repo.wiki.new_page")

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplWikiNew)
		return
	}

	oldWikiName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	newWikiName := wiki_service.UserTitleToWebPath("", form.Title)

	if len(form.Message) == 0 {
		form.Message = ctx.Locale.TrString("repo.editor.update", form.Title)
	}

	if err := wiki_service.EditWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, oldWikiName, newWikiName, form.Content, form.Message); err != nil {
		ctx.ServerError("EditWikiPage", err)
		return
	}

	notify_service.EditWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(newWikiName), form.Message)

	ctx.Redirect(ctx.Repo.RepoLink + "/wiki/" + wiki_service.WebPathToURLPath(newWikiName))
}

// DeleteWikiPagePost delete wiki page
func DeleteWikiPagePost(ctx *context.Context) {
	wikiName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if len(wikiName) == 0 {
		wikiName = "Home"
	}

	if err := wiki_service.DeleteWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, wikiName); err != nil {
		ctx.ServerError("DeleteWikiPage", err)
		return
	}

	notify_service.DeleteWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(wikiName))

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/wiki/")
}
