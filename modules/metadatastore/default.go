// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package metadatastore

import (
	"github.com/gitjet-ru/core-scm/modules/metadatastore/postgres"
)

// Default returns the supported metadata backend (PostgreSQL).
func Default() Backend {
	return postgres.New()
}

// FromConfig returns the metadata backend for the current configuration (PostgreSQL only).
func FromConfig() Backend {
	return Default()
}
