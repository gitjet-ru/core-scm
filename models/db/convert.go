// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"xorm.io/xorm"
	"xorm.io/xorm/convert"
)

// ConvertDatabaseTable is a no-op in PostgreSQL-only mode.
func ConvertDatabaseTable() error {
	return nil
}

// ConvertVarcharToNVarchar is a no-op in PostgreSQL-only mode.
func ConvertVarcharToNVarchar() error {
	return nil
}

// CellToInt converts a xorm.Cell field value to an int value
func CellToInt[T ~int | int64](cell xorm.Cell, def T) (ret T, has bool, err error) {
	if *cell == nil {
		return def, false, nil
	}
	val, err := convert.AsInt64(*cell)
	if err != nil {
		return def, false, err
	}
	return T(val), true, err
}
