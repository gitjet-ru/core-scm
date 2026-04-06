// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gitjet-ru/core-scm/models/unittest"
	user_model "github.com/gitjet-ru/core-scm/models/user"
	"github.com/gitjet-ru/core-scm/modules/setting"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

var NewFixturesLoaderVendor = func(e *xorm.Engine, opts unittest.FixturesOptions) (unittest.FixturesLoader, error) {
	return nil, nil //nolint:nilnil // no vendor fixtures loader configured
}

func TestMain(m *testing.M) {
	setting.SetupGiteaTestEnv()
	os.Exit(m.Run())
}

func prepareTestFixturesLoaders(t testing.TB) unittest.FixturesOptions {
	_ = user_model.User{}
	giteaRoot := setting.GetGiteaTestSourceRoot()
	opts := unittest.FixturesOptions{Dir: filepath.Join(giteaRoot, "models", "fixtures"), Files: []string{
		"user.yml",
	}}
	require.NoError(t, unittest.CreateTestEngine(opts))
	return opts
}

func TestFixturesLoader(t *testing.T) {
	opts := prepareTestFixturesLoaders(t)
	loaderInternal, err := unittest.NewFixturesLoader(unittest.GetXORMEngine(), opts)
	require.NoError(t, err)
	loaderVendor, err := NewFixturesLoaderVendor(unittest.GetXORMEngine(), opts)
	require.NoError(t, err)
	t.Run("Internal", func(t *testing.T) {
		require.NoError(t, loaderInternal.Load())
		require.NoError(t, loaderInternal.Load())
	})
	t.Run("Vendor", func(t *testing.T) {
		if loaderVendor == nil {
			t.Skip()
		}
		require.NoError(t, loaderVendor.Load())
		require.NoError(t, loaderVendor.Load())
	})
}

func BenchmarkFixturesLoader(b *testing.B) {
	opts := prepareTestFixturesLoaders(b)
	require.NoError(b, unittest.CreateTestEngine(opts))
	loaderInternal, err := unittest.NewFixturesLoader(unittest.GetXORMEngine(), opts)
	require.NoError(b, err)
	loaderVendor, err := NewFixturesLoaderVendor(unittest.GetXORMEngine(), opts)
	require.NoError(b, err)

	// BenchmarkFixturesLoader/Vendor
	// BenchmarkFixturesLoader/Vendor-12         	    1696	    719416 ns/op
	// BenchmarkFixturesLoader/Internal
	// BenchmarkFixturesLoader/Internal-12       	    1746	    670457 ns/op
	b.Run("Internal", func(b *testing.B) {
		for b.Loop() {
			require.NoError(b, loaderInternal.Load())
		}
	})
	b.Run("Vendor", func(b *testing.B) {
		if loaderVendor == nil {
			b.Skip()
		}
		for b.Loop() {
			require.NoError(b, loaderVendor.Load())
		}
	})
}
