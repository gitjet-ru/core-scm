// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gitjet-ru/core-scm/models/unittest"
	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/gitjet-ru/core-scm/modules/metadatastore"
	"github.com/gitjet-ru/core-scm/modules/setting"
	"github.com/gitjet-ru/core-scm/modules/tempdir"
	"github.com/gitjet-ru/core-scm/modules/testlogger"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// FIXME: this file shouldn't be in a normal package, it should only be compiled for tests

func newXORMEngine(t *testing.T) (*xorm.Engine, error) {
	if err := metadatastore.Default().Init(t.Context()); err != nil {
		return nil, err
	}
	x := unittest.GetXORMEngine()
	return x, nil
}

func deleteDB() error {
	dbConn, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/?sslmode=%s",
		setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.SSLMode))
	if err != nil {
		return err
	}
	defer dbConn.Close()

	if _, err = dbConn.Exec("DROP DATABASE IF EXISTS " + setting.Database.Name); err != nil {
		return err
	}

	if _, err = dbConn.Exec("CREATE DATABASE " + setting.Database.Name); err != nil {
		return err
	}
	_ = dbConn.Close()

	if len(setting.Database.Schema) != 0 {
		dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
		if err != nil {
			return err
		}
		defer dbConn.Close()

		schrows, err := dbConn.Query(fmt.Sprintf("SELECT 1 FROM information_schema.schemata WHERE schema_name = '%s'", setting.Database.Schema))
		if err != nil {
			return err
		}
		defer schrows.Close()

		if !schrows.Next() {
			if _, err = dbConn.Exec("CREATE SCHEMA " + setting.Database.Schema); err != nil {
				return err
			}
		}

		if _, err = dbConn.Exec(fmt.Sprintf(`ALTER USER "%s" SET search_path = %s`, setting.Database.User, setting.Database.Schema)); err != nil {
			return err
		}
	}

	return nil
}

// PrepareTestEnv prepares the test environment and reset the database. The skip parameter should usually be 0.
// Provide models to be sync'd with the database - in particular any models you expect fixtures to be loaded from.
//
// fixtures in `models/migrations/fixtures/<TestName>` will be loaded automatically
func PrepareTestEnv(t *testing.T, skip int, syncModels ...any) (*xorm.Engine, func()) {
	t.Helper()
	ourSkip := 2
	ourSkip += skip
	deferFn := testlogger.PrintCurrentTest(t, ourSkip)
	require.NoError(t, unittest.SyncDirs(filepath.Join(filepath.Dir(setting.AppPath), "tests/gitea-repositories-meta"), setting.RepoRootPath))

	if err := deleteDB(); err != nil {
		t.Fatalf("unable to reset database: %v", err)
		return nil, deferFn
	}

	x, err := newXORMEngine(t)
	require.NoError(t, err)
	if x != nil {
		oldDefer := deferFn
		deferFn = func() {
			oldDefer()
			if err := x.Close(); err != nil {
				t.Errorf("error during close: %v", err)
			}
			if err := deleteDB(); err != nil {
				t.Errorf("unable to reset database: %v", err)
			}
		}
	}
	if err != nil {
		return x, deferFn
	}

	if len(syncModels) > 0 {
		if err := x.Sync(syncModels...); err != nil {
			t.Errorf("error during sync: %v", err)
			return x, deferFn
		}
	}

	fixturesDir := filepath.Join(filepath.Dir(setting.AppPath), "models", "migrations", "fixtures", t.Name())

	if _, err := os.Stat(fixturesDir); err == nil {
		t.Logf("initializing fixtures from: %s", fixturesDir)
		if err := unittest.InitFixtures(
			unittest.FixturesOptions{
				Dir: fixturesDir,
			}, x); err != nil {
			t.Errorf("error whilst initializing fixtures from %s: %v", fixturesDir, err)
			return x, deferFn
		}
		if err := unittest.LoadFixtures(); err != nil {
			t.Errorf("error whilst loading fixtures from %s: %v", fixturesDir, err)
			return x, deferFn
		}
	} else if !os.IsNotExist(err) {
		t.Errorf("unexpected error whilst checking for existence of fixtures: %v", err)
	} else {
		t.Logf("no fixtures found in: %s", fixturesDir)
	}

	return x, deferFn
}

func LoadTableSchemasMap(t *testing.T, x *xorm.Engine) map[string]*schemas.Table {
	tables, err := x.DBMetas()
	require.NoError(t, err)
	tableMap := make(map[string]*schemas.Table)
	for _, table := range tables {
		tableMap[table.Name] = table
	}
	return tableMap
}

func mainTest(m *testing.M) int {
	testlogger.Init()

	tmpDataPath, cleanup, err := tempdir.OsTempDir("gitea-test").MkdirTempRandom("data")
	if err != nil {
		testlogger.Panicf("Unable to create temporary data path %v\n", err)
	}
	defer cleanup()

	setting.AppDataPath = tmpDataPath

	unittest.InitSettingsForTesting()
	if err = git.InitFull(); err != nil {
		testlogger.Panicf("Unable to InitFull: %v\n", err)
	}
	setting.LoadDBSetting()
	setting.InitLoggersForTest()
	return m.Run()
}

func MainTest(m *testing.M) {
	os.Exit(mainTest(m))
}
