// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"html/template"
	"testing"

	pull_model "github.com/gitjet-ru/core-scm/models/pull"
	"github.com/gitjet-ru/core-scm/modules/fileicon"
	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/gitjet-ru/core-scm/services/gitdiff"

	"github.com/stretchr/testify/assert"
)

func TestTransformDiffTreeForWeb(t *testing.T) {
	renderedIconPool := fileicon.NewRenderedIconPool()
	ret := transformDiffTreeForWeb(renderedIconPool, &gitdiff.DiffTree{Files: []*gitdiff.DiffTreeRecord{
		{
			Status:   "changed",
			HeadPath: "dir-a/dir-a-x/file-deep",
			HeadMode: git.EntryModeBlob,
		},
		{
			Status:   "added",
			HeadPath: "file1",
			HeadMode: git.EntryModeBlob,
		},
	}}, map[string]pull_model.ViewedState{
		"dir-a/dir-a-x/file-deep": pull_model.Viewed,
	})

	mockIconForFile := func(id string) template.HTML {
		return template.HTML(`<svg class="svg git-entry-icon octicon-file" width="16" height="16" aria-hidden="true"><use href="#` + id + `"></use></svg>`)
	}
	assert.Equal(t, WebDiffFileTree{
		TreeRoot: WebDiffFileItem{
			Children: []*WebDiffFileItem{
				{
					EntryMode:   "tree",
					DisplayName: "dir-a/dir-a-x",
					FullName:    "dir-a/dir-a-x",
					Children: []*WebDiffFileItem{
						{
							EntryMode:   "",
							DisplayName: "file-deep",
							FullName:    "dir-a/dir-a-x/file-deep",
							NameHash:    "4acf7eef1c943a09e9f754e93ff190db8583236b",
							DiffStatus:  "changed",
							IsViewed:    true,
							FileIcon:    mockIconForFile(`svg-mfi-file`),
						},
					},
				},
				{
					EntryMode:   "",
					DisplayName: "file1",
					FullName:    "file1",
					NameHash:    "60b27f004e454aca81b0480209cce5081ec52390",
					DiffStatus:  "added",
					FileIcon:    mockIconForFile(`svg-mfi-file`),
				},
			},
		},
	}, ret)
}

func TestTransformDiffTreeForWeb_sortsDirsBeforeFiles(t *testing.T) {
	renderedIconPool := fileicon.NewRenderedIconPool()
	// Input order: file at repo root first, nested file second — output must list the directory branch first.
	ret := transformDiffTreeForWeb(renderedIconPool, &gitdiff.DiffTree{Files: []*gitdiff.DiffTreeRecord{
		{
			Status:   "added",
			HeadPath: "file1",
			HeadMode: git.EntryModeBlob,
		},
		{
			Status:   "modified",
			HeadPath: "dir-a/dir-a-x/file-deep",
			HeadMode: git.EntryModeBlob,
		},
	}}, nil)

	assert.Len(t, ret.TreeRoot.Children, 2)
	assert.Equal(t, "tree", ret.TreeRoot.Children[0].EntryMode)
	assert.Equal(t, "dir-a/dir-a-x", ret.TreeRoot.Children[0].FullName)
	assert.Equal(t, "", ret.TreeRoot.Children[1].EntryMode)
	assert.Equal(t, "file1", ret.TreeRoot.Children[1].FullName)
}
