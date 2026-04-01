// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gitjet-ru/core-scm/modules/git/gitcmd"
)

const notRegularFileMode = os.ModeSymlink | os.ModeNamedPipe | os.ModeSocket | os.ModeDevice | os.ModeCharDevice | os.ModeIrregular

// CalcRepositorySize returns the disk consumption for a given path
func CalcRepositorySize(repo Repository) (int64, error) {
	if isRemoteBackendEnabled() {
		stdout, _, err := RunCmdString(context.Background(), repo, gitcmd.NewCommand("count-objects", "-v"))
		if err != nil {
			return 0, err
		}
		// count-objects reports size in KiB.
		var sizeKB int64
		for _, line := range strings.Split(stdout, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "size-pack:") || strings.HasPrefix(line, "size:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) != 2 {
					continue
				}
				n, convErr := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
				if convErr == nil {
					sizeKB += n
				}
			}
		}
		return sizeKB * 1024, nil
	}

	basePath := repoPath(repo)
	var size int64
	err := filepath.WalkDir(basePath, func(_ string, entry os.DirEntry, err error) error {
		if os.IsNotExist(err) { // ignore the error because some files (like temp/lock file) may be deleted during traversing.
			return nil
		} else if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if os.IsNotExist(err) { // ignore the error as above
			return nil
		} else if err != nil {
			return err
		}
		if (info.Mode() & notRegularFileMode) == 0 {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
