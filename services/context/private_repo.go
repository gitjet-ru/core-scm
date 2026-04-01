package context

import (
	"fmt"

	"github.com/gitjet-ru/core-scm/modules/git"
	repo_model "github.com/gitjet-ru/core-scm/models/repo"
)

// OpenPrivateGitRepo opens repository for private hook handlers.
func OpenPrivateGitRepo(_ *PrivateContext, repo *repo_model.Repository) (*git.Repository, error) {
	return nil, fmt.Errorf("local mirror open is disabled in mirrorless hard-cut for %s", repo.RelativePath())
}

