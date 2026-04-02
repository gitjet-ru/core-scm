package git

import (
	"context"
	"fmt"
)

// NewRemoteRepositoryForReads creates a repository adapter intended for git-storage RPC reads.
//
// The returned repository doesn't require local git objects to exist; remote read helpers
// in this package will use repo.Path as a "repo relative path" identifier.
func NewRemoteRepositoryForReads(ctx context.Context, repoRelativePath, objectFormatName string) (*Repository, error) {
	objectFormat := ObjectFormatFromName(objectFormatName)
	if objectFormat == nil {
		return nil, fmt.Errorf("invalid object format: %s", objectFormatName)
	}

	return &Repository{
		Path:     repoRelativePath,
		tagCache: newObjectCache[*Tag](),
		Ctx:      ctx,
		objectFormat: objectFormat,
	}, nil
}

