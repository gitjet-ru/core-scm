// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"bufio"
	"context"
	"errors"
	"html/template"
	"strings"

	"github.com/gitjet-ru/core-scm/models/perm/access"
	"github.com/gitjet-ru/core-scm/models/repo"
	"github.com/gitjet-ru/core-scm/models/unit"
	"github.com/gitjet-ru/core-scm/modules/charset"
	"github.com/gitjet-ru/core-scm/modules/gitrepo"
	"github.com/gitjet-ru/core-scm/modules/indexer/code"
	"github.com/gitjet-ru/core-scm/modules/markup"
	"github.com/gitjet-ru/core-scm/modules/setting"
	"github.com/gitjet-ru/core-scm/modules/util"
	gitea_context "github.com/gitjet-ru/core-scm/services/context"
)

func renderRepoFileCodePreview(ctx context.Context, opts markup.RenderCodePreviewOptions) (template.HTML, error) {
	opts.LineStop = max(opts.LineStop, opts.LineStart)
	lineCount := opts.LineStop - opts.LineStart + 1
	if lineCount <= 0 || lineCount > 140 /* GitHub at most show 140 lines */ {
		lineCount = 10
		opts.LineStop = opts.LineStart + lineCount
	}

	dbRepo, err := repo.GetRepositoryByOwnerAndName(ctx, opts.OwnerName, opts.RepoName)
	if err != nil {
		return "", err
	}

	webCtx := gitea_context.GetWebContext(ctx)
	if webCtx == nil {
		return "", errors.New("context is not a web context")
	}
	doer := webCtx.Doer

	perms, err := access.GetUserRepoPermission(ctx, dbRepo, doer)
	if err != nil {
		return "", err
	}
	if !perms.CanRead(unit.TypeCode) {
		return "", util.ErrPermissionDenied
	}
	maxBlobSize := int32(setting.UI.MaxDisplayFileSize)
	if setting.UI.MaxDisplayFileSize > int64(^uint32(0)>>1) {
		maxBlobSize = int32(^uint32(0) >> 1)
	}
	blob, err := gitrepo.RemoteGetBlobForAPI(ctx, dbRepo, opts.CommitID, opts.FilePath, maxBlobSize)
	if err != nil {
		return "", err
	}
	if blob.GetTruncated() {
		return "", errors.New("file is too large")
	}
	return renderCodePreviewFromReader(webCtx, dbRepo, opts, bufio.NewReader(strings.NewReader(string(blob.GetContent()))), "")
}

func renderCodePreviewFromReader(webCtx *gitea_context.Context, dbRepo *repo.Repository, opts markup.RenderCodePreviewOptions, reader *bufio.Reader, language string) (template.HTML, error) {
	opts.LineStop = max(opts.LineStop, opts.LineStart)
	lineCount := opts.LineStop - opts.LineStart + 1
	if lineCount <= 0 || lineCount > 140 {
		lineCount = 10
		opts.LineStop = opts.LineStart + lineCount
	}

	for i := 1; i < opts.LineStart; i++ {
		if _, err := reader.ReadBytes('\n'); err != nil {
			return "", err
		}
	}

	lineNums := make([]int, 0, lineCount)
	lineCodes := make([]string, 0, lineCount)
	for i := opts.LineStart; i <= opts.LineStop; i++ {
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			break
		}

		lineNums = append(lineNums, i)
		lineCodes = append(lineCodes, line)
	}
	realLineStop := max(opts.LineStart, opts.LineStart+len(lineNums)-1)
	highlightLines := code.HighlightSearchResultCode(opts.FilePath, language, lineNums, strings.Join(lineCodes, ""))

	escapeStatus := &charset.EscapeStatus{}
	lineEscapeStatus := make([]*charset.EscapeStatus, len(highlightLines))
	for i, hl := range highlightLines {
		lineEscapeStatus[i], hl.FormattedContent = charset.EscapeControlHTML(hl.FormattedContent, webCtx.Base.Locale, charset.RuneNBSP)
		escapeStatus = escapeStatus.Or(lineEscapeStatus[i])
	}

	return webCtx.RenderToHTML("base/markup_codepreview", map[string]any{
		"FullURL":          opts.FullURL,
		"FilePath":         opts.FilePath,
		"LineStart":        opts.LineStart,
		"LineStop":         realLineStop,
		"RepoName":         opts.RepoName,
		"RepoLink":         dbRepo.Link(),
		"CommitID":         opts.CommitID,
		"HighlightLines":   highlightLines,
		"EscapeStatus":     escapeStatus,
		"LineEscapeStatus": lineEscapeStatus,
	})
}
