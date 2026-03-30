// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"

	activities_model "github.com/gitjet-ru/core-scm/models/activities"
	repo_model "github.com/gitjet-ru/core-scm/models/repo"
	"github.com/gitjet-ru/core-scm/models/unittest"
	user_model "github.com/gitjet-ru/core-scm/models/user"
	"github.com/gitjet-ru/core-scm/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestToNotificationThreadIncludesRepoForAccessibleUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	n := newRepoNotification(t, 1, 4)
	thread := ToNotificationThread(t.Context(), n)

	if assert.NotNil(t, thread.Repository) {
		assert.Equal(t, n.Repository.FullName(), thread.Repository.FullName)
		assert.Nil(t, thread.Repository.Permissions)
	}
}

func TestToNotificationThreadOmitsRepoWhenAccessRevoked(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	n := newRepoNotification(t, 2, 4)
	thread := ToNotificationThread(t.Context(), n)

	assert.Nil(t, thread.Repository)
}

func newRepoNotification(t *testing.T, repoID, userID int64) *activities_model.Notification {
	t.Helper()

	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
	assert.NoError(t, repo.LoadOwner(ctx))
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})

	return &activities_model.Notification{
		ID:          repoID*1000 + userID,
		UserID:      user.ID,
		RepoID:      repo.ID,
		Status:      activities_model.NotificationStatusUnread,
		Source:      activities_model.NotificationSourceRepository,
		UpdatedUnix: timeutil.TimeStampNow(),
		Repository:  repo,
		User:        user,
	}
}
