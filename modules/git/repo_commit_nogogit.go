// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"errors"
	"io"
	"strings"
	"time"

	"github.com/gitjet-ru/core-scm/modules/git/gitcmd"
	"github.com/gitjet-ru/core-scm/modules/log"
)

// ResolveReference resolves a name to a reference
func (repo *Repository) ResolveReference(name string) (string, error) {
	stdout, _, err := gitcmd.NewCommand("show-ref", "--hash").
		AddDynamicArguments(name).
		WithDir(repo.Path).
		RunStdString(repo.Ctx)
	if err != nil {
		if strings.Contains(err.Error(), "not a valid ref") {
			return "", ErrNotExist{name, ""}
		}
		return "", err
	}
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return "", ErrNotExist{name, ""}
	}

	return stdout, nil
}

// GetRefCommitID returns the last commit ID string of given reference (branch or tag).
func (repo *Repository) GetRefCommitID(name string) (string, error) {
	if remoteReadRepo(repo) {
		commit, err := remoteGetCommit(repo.Ctx, repo.Path, strings.TrimSpace(name))
		if err != nil {
			return "", err
		}
		if commit == nil {
			return "", ErrNotExist{name, ""}
		}
		return commit.GetId(), nil
	}
	batch, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		return "", err
	}
	defer cancel()
	info, err := batch.QueryInfo(name)
	if IsErrNotExist(err) {
		return "", ErrNotExist{name, ""}
	} else if err != nil {
		return "", err
	}
	return info.ID, nil
}

func (repo *Repository) getCommit(id ObjectID) (*Commit, error) {
	if remoteReadRepo(repo) {
		info, err := remoteGetCommit(repo.Ctx, repo.Path, id.String())
		if err != nil {
			return nil, err
		}
		if info == nil {
			return nil, ErrNotExist{ID: id.String()}
		}

		treeID, err := NewIDFromString(info.GetTreeId())
		if err != nil {
			return nil, err
		}
		commitID, err := NewIDFromString(info.GetId())
		if err != nil {
			return nil, err
		}

		authorWhen := time.Unix(info.GetAuthorUnix(), 0)
		committerWhen := time.Unix(info.GetCommitterUnix(), 0)

		message := info.GetSubject()
		if body := strings.TrimSpace(info.GetBody()); body != "" {
			message += "\n\n" + body
		}

		parents := make([]ObjectID, 0, len(info.GetParentIds()))
		for _, pid := range info.GetParentIds() {
			if strings.TrimSpace(pid) == "" {
				continue
			}
			p, err := NewIDFromString(pid)
			if err != nil {
				return nil, err
			}
			parents = append(parents, p)
		}

		return &Commit{
			Tree: Tree{
				TreeCommon: TreeCommon{
					ID:   treeID,
					repo: repo,
				},
			},
			ID: commitID,
			Author: &Signature{
				Name:  info.GetAuthorName(),
				Email: info.GetAuthorEmail(),
				When:  authorWhen,
			},
			Committer: &Signature{
				Name:  info.GetCommitterName(),
				Email: info.GetCommitterEmail(),
				When:  committerWhen,
			},
			CommitMessage: message,
			Parents:        parents,
		}, nil
	}
	batch, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	return repo.getCommitWithBatch(batch, id)
}

func (repo *Repository) getCommitWithBatch(batch CatFileBatch, id ObjectID) (*Commit, error) {
	info, rd, err := batch.QueryContent(id.String())
	if err != nil {
		if errors.Is(err, io.EOF) || IsErrNotExist(err) {
			return nil, ErrNotExist{ID: id.String()}
		}
		return nil, err
	}

	switch info.Type {
	case "missing":
		return nil, ErrNotExist{ID: id.String()}
	case "tag":
		// then we need to parse the tag
		// and load the commit
		data, err := io.ReadAll(io.LimitReader(rd, info.Size))
		if err != nil {
			return nil, err
		}
		_, err = rd.Discard(1)
		if err != nil {
			return nil, err
		}
		tag, err := parseTagData(id.Type(), data)
		if err != nil {
			return nil, err
		}
		return repo.getCommitWithBatch(batch, tag.Object)
	case "commit":
		commit, err := CommitFromReader(repo, id, io.LimitReader(rd, info.Size))
		if err != nil {
			return nil, err
		}
		_, err = rd.Discard(1)
		if err != nil {
			return nil, err
		}

		return commit, nil
	default:
		log.Debug("Unknown cat-file object type: %s", info.Type)
		if err := DiscardFull(rd, info.Size+1); err != nil {
			return nil, err
		}
		return nil, ErrNotExist{
			ID: id.String(),
		}
	}
}

// ConvertToGitID returns a GitHash object from a potential ID string
func (repo *Repository) ConvertToGitID(commitID string) (ObjectID, error) {
	objectFormat, err := repo.GetObjectFormat()
	if err != nil {
		return nil, err
	}
	if len(commitID) == objectFormat.FullLength() && objectFormat.IsValid(commitID) {
		ID, err := NewIDFromString(commitID)
		if err == nil {
			return ID, nil
		}
	}

	if remoteReadRepo(repo) {
		info, err := remoteGetCommit(repo.Ctx, repo.Path, strings.TrimSpace(commitID))
		if err != nil {
			return nil, err
		}
		if info == nil {
			return nil, ErrNotExist{commitID, ""}
		}
		return MustIDFromString(info.GetId()), nil
	}

	batch, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	info, err := batch.QueryInfo(commitID)
	if err != nil {
		if IsErrNotExist(err) {
			return nil, ErrNotExist{commitID, ""}
		}
		return nil, err
	}

	return MustIDFromString(info.ID), nil
}
