// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon

import "github.com/gitjet-ru/core-scm/modules/git"

type EntryInfo struct {
	BaseName      string
	EntryMode     git.EntryMode
	SymlinkToMode git.EntryMode
	IsOpen        bool
}

func EntryInfoFromGitTreeEntry(commit *git.Commit, fullPath string, gitEntry *git.TreeEntry) *EntryInfo {
	ret := &EntryInfo{BaseName: gitEntry.Name(), EntryMode: gitEntry.Mode()}
	if gitEntry.IsLink() {
		if res, err := git.EntryFollowLink(commit, fullPath, gitEntry); err == nil && res.TargetEntry.IsDir() {
			ret.SymlinkToMode = res.TargetEntry.Mode()
		}
	}
	return ret
}

// EntryInfoFromIndexedName builds icon metadata from branch-tree index fields (no git tree walk).
func EntryInfoFromIndexedName(name string, isDir, isLink, isSubmodule bool) *EntryInfo {
	var mode git.EntryMode
	switch {
	case isSubmodule:
		mode = git.EntryModeCommit
	case isDir:
		mode = git.EntryModeTree
	case isLink:
		mode = git.EntryModeSymlink
	default:
		mode = git.EntryModeBlob
	}
	return &EntryInfo{BaseName: name, EntryMode: mode}
}

func EntryInfoFolder() *EntryInfo {
	return &EntryInfo{EntryMode: git.EntryModeTree}
}

func EntryInfoFolderOpen() *EntryInfo {
	return &EntryInfo{EntryMode: git.EntryModeTree, IsOpen: true}
}
