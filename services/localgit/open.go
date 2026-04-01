package localgit

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gitjet-ru/core-scm/modules/git"
)

// OpenRepository opens local git repository only for non-mirrorless backends.
func OpenRepository(ctx context.Context, path string) (*git.Repository, error) {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("GIT_STORAGE_BACKEND")))
	if backend == "remote" || backend == "shadow" {
		return nil, fmt.Errorf("local git repository open is disabled in mirrorless hard-cut: %s", path)
	}
	return git.OpenRepository(ctx, path)
}
