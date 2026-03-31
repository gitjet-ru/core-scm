// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package tests

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gitjet-ru/core-scm/models/db"
	packages_model "github.com/gitjet-ru/core-scm/models/packages"
	"github.com/gitjet-ru/core-scm/models/unittest"
	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/gitjet-ru/core-scm/modules/graceful"
	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/setting"
	"github.com/gitjet-ru/core-scm/modules/storage"
	"github.com/gitjet-ru/core-scm/modules/testlogger"
	"github.com/gitjet-ru/core-scm/modules/util"
	"github.com/gitjet-ru/core-scm/routers"

	"github.com/stretchr/testify/assert"
)

func InitTest() {
	testlogger.Init()
	unittest.InitSettingsForTesting()
	setting.Repository.DefaultBranch = "master" // many test code still assume that default branch is called "master"

	if err := git.InitFull(); err != nil {
		log.Fatal("git.InitOnceWithSync: %v", err)
	}

	setting.LoadDBSetting()
	if err := storage.Init(); err != nil {
		testlogger.Panicf("Init storage failed: %v\n", err)
	}

	unixSocket := len(setting.Database.Host) > 0 && setting.Database.Host[0] == '/'

	var adminDB *sql.DB
	var err error
	if unixSocket {
		adminDB, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@/?sslmode=%s&host=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.SSLMode, setting.Database.Host))
	} else {
		adminDB, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/?sslmode=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.SSLMode))
	}
	if err != nil {
		log.Fatal("sql.Open: %v", err)
	}
	if _, err = adminDB.Exec("CREATE DATABASE " + setting.Database.Name); err != nil {
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "already exists") {
			_ = adminDB.Close()
			log.Fatal("db.Exec: CREATE DATABASE: %v", err)
		}
	}
	_ = adminDB.Close()

	if len(setting.Database.Schema) == 0 {
		return
	}

	var sqlDB *sql.DB
	if unixSocket {
		sqlDB, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@/%s?sslmode=%s&host=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.Name, setting.Database.SSLMode, setting.Database.Host))
	} else {
		sqlDB, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
	}
	if err != nil {
		log.Fatal("sql.Open: %v", err)
	}
	defer sqlDB.Close()

	schrows, err := sqlDB.Query(fmt.Sprintf("SELECT 1 FROM information_schema.schemata WHERE schema_name = '%s'", setting.Database.Schema))
	if err != nil {
		log.Fatal("db.Query: %v", err)
	}
	defer schrows.Close()

	if !schrows.Next() {
		if _, err = sqlDB.Exec("CREATE SCHEMA " + setting.Database.Schema); err != nil {
			log.Fatal("db.Exec: CREATE SCHEMA: %v", err)
		}
	}

	routers.InitWebInstalled(graceful.GetManager().HammerContext())
}

func PrepareAttachmentsStorage(t testing.TB) {
	// prepare attachments directory and files
	assert.NoError(t, storage.Clean(storage.Attachments))

	s, err := storage.NewStorage(setting.LocalStorageType, &setting.Storage{
		Path: filepath.Join(filepath.Dir(setting.AppPath), "tests", "testdata", "data", "attachments"),
	})
	assert.NoError(t, err)
	assert.NoError(t, s.IterateObjects("", func(p string, obj storage.Object) error {
		_, err = storage.Copy(storage.Attachments, p, s, p)
		return err
	}))
}

func PrepareGitRepoDirectory(t testing.TB) {
	if !assert.NotEmpty(t, setting.RepoRootPath) {
		return
	}
	assert.NoError(t, unittest.SyncDirs(filepath.Join(filepath.Dir(setting.AppPath), "tests/gitea-repositories-meta"), setting.RepoRootPath))
}

func PrepareArtifactsStorage(t testing.TB) {
	// prepare actions artifacts directory and files
	assert.NoError(t, storage.Clean(storage.ActionsArtifacts))

	s, err := storage.NewStorage(setting.LocalStorageType, &setting.Storage{
		Path: filepath.Join(filepath.Dir(setting.AppPath), "tests", "testdata", "data", "artifacts"),
	})
	assert.NoError(t, err)
	assert.NoError(t, s.IterateObjects("", func(p string, obj storage.Object) error {
		_, err = storage.Copy(storage.ActionsArtifacts, p, s, p)
		return err
	}))
}

func PrepareLFSStorage(t testing.TB) {
	// load LFS object fixtures
	// (LFS storage can be on any of several backends, including remote servers, so init it with the storage API)
	lfsFixtures, err := storage.NewStorage(setting.LocalStorageType, &setting.Storage{
		Path: filepath.Join(filepath.Dir(setting.AppPath), "tests/gitea-lfs-meta"),
	})
	assert.NoError(t, err)
	assert.NoError(t, storage.Clean(storage.LFS))
	assert.NoError(t, lfsFixtures.IterateObjects("", func(path string, _ storage.Object) error {
		_, err := storage.Copy(storage.LFS, path, lfsFixtures, path)
		return err
	}))
}

func PrepareCleanPackageData(t testing.TB) {
	// clear all package data
	assert.NoError(t, db.TruncateBeans(t.Context(),
		&packages_model.Package{},
		&packages_model.PackageVersion{},
		&packages_model.PackageFile{},
		&packages_model.PackageBlob{},
		&packages_model.PackageProperty{},
		&packages_model.PackageBlobUpload{},
		&packages_model.PackageCleanupRule{},
	))
	assert.NoError(t, storage.Clean(storage.Packages))
}

func PrepareTestEnv(t testing.TB, skip ...int) func() {
	t.Helper()
	deferFn := PrintCurrentTest(t, util.OptionalArg(skip)+1)

	// load database fixtures
	assert.NoError(t, unittest.LoadFixtures())

	// do not add more Prepare* functions here, only call necessary ones in the related test functions
	PrepareGitRepoDirectory(t)
	PrepareLFSStorage(t)
	PrepareCleanPackageData(t)
	return deferFn
}

func PrintCurrentTest(t testing.TB, skip ...int) func() {
	t.Helper()
	return testlogger.PrintCurrentTest(t, util.OptionalArg(skip)+1)
}
