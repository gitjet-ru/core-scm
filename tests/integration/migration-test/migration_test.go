// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/gitjet-ru/core-scm/models/db"
	"github.com/gitjet-ru/core-scm/models/migrations"
	migrate_base "github.com/gitjet-ru/core-scm/models/migrations/base"
	"github.com/gitjet-ru/core-scm/models/unittest"
	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/metadatastore"
	"github.com/gitjet-ru/core-scm/modules/setting"
	"github.com/gitjet-ru/core-scm/modules/testlogger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

var currentEngine *xorm.Engine

func initMigrationTest(t *testing.T) func() {
	testlogger.Init()
	unittest.InitSettingsForTesting()

	assert.NotEmpty(t, setting.RepoRootPath)
	assert.NoError(t, unittest.SyncDirs(filepath.Join(filepath.Dir(setting.AppPath), "tests/gitea-repositories-meta"), setting.RepoRootPath))
	assert.NoError(t, git.InitFull())
	setting.LoadDBSetting()
	setting.InitLoggersForTest()

	return testlogger.PrintCurrentTest(t, 2)
}

func availableVersions() ([]string, error) {
	migrationsDir, err := os.Open("tests/integration/migration-test")
	if err != nil {
		return nil, err
	}
	defer migrationsDir.Close()
	versionRE, err := regexp.Compile("gitea-v(?P<version>.+)\\." + regexp.QuoteMeta(setting.Database.Type.String()) + "\\.sql.gz")
	if err != nil {
		return nil, err
	}

	filenames, err := migrationsDir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	versions := []string{}
	for _, filename := range filenames {
		if versionRE.MatchString(filename) {
			substrings := versionRE.FindStringSubmatch(filename)
			versions = append(versions, substrings[1])
		}
	}
	sort.Strings(versions)
	return versions, nil
}

func readSQLFromFile(version string) (string, error) {
	filename := fmt.Sprintf("tests/integration/migration-test/gitea-v%s.%s.sql.gz", version, setting.Database.Type)

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return "", nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gr.Close()

	buf, err := io.ReadAll(gr)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimPrefix(buf, []byte{'\xef', '\xbb', '\xbf'})), nil
}

func restoreOldDB(t *testing.T, version string) {
	data, err := readSQLFromFile(version)
	require.NoError(t, err)
	require.NotEmpty(t, data, "No data found for %s version: %s", setting.Database.Type, version)

	unixSocket := len(setting.Database.Host) > 0 && setting.Database.Host[0] == '/'

	var dbConn *sql.DB
	if unixSocket {
		dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@/?sslmode=%s&host=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.SSLMode, setting.Database.Host))
	} else {
		dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/?sslmode=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.SSLMode))
	}
	assert.NoError(t, err)
	defer dbConn.Close()

	_, err = dbConn.Exec("DROP DATABASE IF EXISTS " + setting.Database.Name)
	assert.NoError(t, err)

	_, err = dbConn.Exec("CREATE DATABASE " + setting.Database.Name)
	assert.NoError(t, err)
	dbConn.Close()

	if len(setting.Database.Schema) != 0 {
		if unixSocket {
			dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@/%s?sslmode=%s&host=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.Name, setting.Database.SSLMode, setting.Database.Host))
		} else {
			dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
		}
		require.NoError(t, err)
		defer dbConn.Close()

		schrows, err := dbConn.Query(fmt.Sprintf("SELECT 1 FROM information_schema.schemata WHERE schema_name = '%s'", setting.Database.Schema))
		require.NoError(t, err)
		require.NotEmpty(t, schrows)

		if !schrows.Next() {
			_, err = dbConn.Exec("CREATE SCHEMA " + setting.Database.Schema)
			assert.NoError(t, err)
		}
		schrows.Close()

		_, err = dbConn.Exec(fmt.Sprintf(`ALTER USER "%s" SET search_path = %s`, setting.Database.User, setting.Database.Schema))
		assert.NoError(t, err)

		dbConn.Close()
	}

	if unixSocket {
		dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@/%s?sslmode=%s&host=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.Name, setting.Database.SSLMode, setting.Database.Host))
	} else {
		dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
	}
	assert.NoError(t, err)
	defer dbConn.Close()

	_, err = dbConn.Exec(data)
	assert.NoError(t, err)
	dbConn.Close()
}

func wrappedMigrate(ctx context.Context, x *xorm.Engine) error {
	currentEngine = x
	return migrations.Migrate(ctx, x)
}

func doMigrationTest(t *testing.T, version string) {
	defer testlogger.PrintCurrentTest(t)()
	restoreOldDB(t, version)

	setting.InitSQLLoggersForCli(log.INFO)

	err := metadatastore.Default().InitWithMigration(t.Context(), wrappedMigrate)
	assert.NoError(t, err)
	currentEngine.Close()

	beans, _ := db.NamesToBean()

	err = metadatastore.Default().InitWithMigration(t.Context(), func(ctx context.Context, x *xorm.Engine) error {
		currentEngine = x
		return migrate_base.RecreateTables(beans...)(x)
	})
	assert.NoError(t, err)
	currentEngine.Close()

	// We do this a second time to ensure that there is not a problem with retained indices
	err = metadatastore.Default().InitWithMigration(t.Context(), func(ctx context.Context, x *xorm.Engine) error {
		currentEngine = x
		return migrate_base.RecreateTables(beans...)(x)
	})
	assert.NoError(t, err)

	currentEngine.Close()
}

func TestMigrations(t *testing.T) {
	defer initMigrationTest(t)()

	dialect := setting.Database.Type
	versions, err := availableVersions()
	require.NoError(t, err)
	require.NotEmpty(t, versions, "No old database versions available to migration test for %s", dialect)

	for _, version := range versions {
		t.Run(fmt.Sprintf("Migrate-%s-%s", dialect, version), func(t *testing.T) {
			doMigrationTest(t, version)
		})
	}
}
