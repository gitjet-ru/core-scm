// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13

import "xorm.io/xorm"

func SetDefaultPasswordToArgon2(x *xorm.Engine) error {
	_, err := x.Exec("ALTER TABLE `user` ALTER COLUMN passwd_hash_algo SET DEFAULT 'argon2';")
	return err
}
