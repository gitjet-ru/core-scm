// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package stats

import (
	"fmt"

	repo_model "github.com/gitjet-ru/core-scm/models/repo"
	"github.com/gitjet-ru/core-scm/modules/graceful"
	"github.com/gitjet-ru/core-scm/modules/process"
)

// DBIndexer implements Indexer interface to use database's like search
type DBIndexer struct{}

// Index repository status function
func (db *DBIndexer) Index(id int64) error {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().ShutdownContext(), fmt.Sprintf("Stats.DB Index Repo[%d]", id))
	defer finished()

	_, err := repo_model.GetRepositoryByID(ctx, id)
	if err != nil {
		return err
	}
	return nil
}

// Close dummy function
func (db *DBIndexer) Close() {
}
