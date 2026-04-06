// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"errors"

	packages_module "github.com/gitjet-ru/core-scm/modules/packages"
)

func SignPackage(buf *packages_module.HashedBuffer, privateKey string) (*packages_module.HashedBuffer, error) {
	_ = buf
	_ = privateKey
	return nil, errors.New("rpm signing is disabled in this build")
}
