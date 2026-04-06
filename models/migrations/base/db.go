// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/setting"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// RecreateTables will recreate the tables for the provided beans using the newly provided bean definition and move all data to that new table
// WARNING: YOU MUST PROVIDE THE FULL BEAN DEFINITION
func RecreateTables(beans ...any) func(*xorm.Engine) error {
	return func(x *xorm.Engine) error {
		sess := x.NewSession()
		defer sess.Close()
		if err := sess.Begin(); err != nil {
			return err
		}
		sess = sess.StoreEngine("InnoDB")
		for _, bean := range beans {
			log.Info("Recreating Table: %s for Bean: %s", x.TableName(bean), reflect.Indirect(reflect.ValueOf(bean)).Type().Name())
			if err := RecreateTable(sess, bean); err != nil {
				return err
			}
		}
		return sess.Commit()
	}
}

// RecreateTable will recreate the table using the newly provided bean definition and move all data to that new table
// WARNING: YOU MUST PROVIDE THE FULL BEAN DEFINITION
// WARNING: YOU MUST COMMIT THE SESSION AT THE END
func RecreateTable(sess *xorm.Session, bean any) error {
	// TODO: This will not work if there are foreign keys

	tableName := sess.Engine().TableName(bean)
	tempTableName := "tmp_recreate__" + tableName

	// We need to move the old table away and create a new one with the correct columns
	// We will need to do this in stages to prevent data loss
	//
	// First create the temporary table
	if err := sess.Table(tempTableName).CreateTable(bean); err != nil {
		log.Error("Unable to create table %s. Error: %v", tempTableName, err)
		return err
	}

	if err := sess.Table(tempTableName).CreateUniques(bean); err != nil {
		log.Error("Unable to create uniques for table %s. Error: %v", tempTableName, err)
		return err
	}

	if err := sess.Table(tempTableName).CreateIndexes(bean); err != nil {
		log.Error("Unable to create indexes for table %s. Error: %v", tempTableName, err)
		return err
	}

	// Work out the column names from the bean - these are the columns to select from the old table and install into the new table
	table, err := sess.Engine().TableInfo(bean)
	if err != nil {
		log.Error("Unable to get table info. Error: %v", err)

		return err
	}
	newTableColumns := table.Columns()
	if len(newTableColumns) == 0 {
		return errors.New("no columns in new table")
	}
	hasID := false
	for _, column := range newTableColumns {
		hasID = hasID || (column.IsPrimaryKey && column.IsAutoIncrement)
	}

	sqlStringBuilder := &strings.Builder{}
	_, _ = sqlStringBuilder.WriteString("INSERT INTO `")
	_, _ = sqlStringBuilder.WriteString(tempTableName)
	_, _ = sqlStringBuilder.WriteString("` (`")
	_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Name)
	_, _ = sqlStringBuilder.WriteString("`")
	for _, column := range newTableColumns[1:] {
		_, _ = sqlStringBuilder.WriteString(", `")
		_, _ = sqlStringBuilder.WriteString(column.Name)
		_, _ = sqlStringBuilder.WriteString("`")
	}
	_, _ = sqlStringBuilder.WriteString(")")
	_, _ = sqlStringBuilder.WriteString(" SELECT ")
	if newTableColumns[0].Default != "" {
		_, _ = sqlStringBuilder.WriteString("COALESCE(`")
		_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Name)
		_, _ = sqlStringBuilder.WriteString("`, ")
		_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Default)
		_, _ = sqlStringBuilder.WriteString(")")
	} else {
		_, _ = sqlStringBuilder.WriteString("`")
		_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Name)
		_, _ = sqlStringBuilder.WriteString("`")
	}

	for _, column := range newTableColumns[1:] {
		if column.Default != "" {
			_, _ = sqlStringBuilder.WriteString(", COALESCE(`")
			_, _ = sqlStringBuilder.WriteString(column.Name)
			_, _ = sqlStringBuilder.WriteString("`, ")
			_, _ = sqlStringBuilder.WriteString(column.Default)
			_, _ = sqlStringBuilder.WriteString(")")
		} else {
			_, _ = sqlStringBuilder.WriteString(", `")
			_, _ = sqlStringBuilder.WriteString(column.Name)
			_, _ = sqlStringBuilder.WriteString("`")
		}
	}
	_, _ = sqlStringBuilder.WriteString(" FROM `")
	_, _ = sqlStringBuilder.WriteString(tableName)
	_, _ = sqlStringBuilder.WriteString("`")

	if _, err := sess.Exec(sqlStringBuilder.String()); err != nil {
		log.Error("Unable to set copy data in to temp table %s. Error: %v", tempTableName, err)
		return err
	}

	{
		var originalSequences []string
		type sequenceData struct {
			LastValue int  `xorm:"'last_value'"`
			IsCalled  bool `xorm:"'is_called'"`
		}
		sequenceMap := map[string]sequenceData{}

		schema := sess.Engine().Dialect().URI().Schema
		sess.Engine().SetSchema("")
		if err := sess.Table("information_schema.sequences").Cols("sequence_name").Where("sequence_name LIKE ? || '_%' AND sequence_catalog = ?", tableName, setting.Database.Name).Find(&originalSequences); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
		sess.Engine().SetSchema(schema)

		for _, sequence := range originalSequences {
			sequenceData := sequenceData{}
			if _, err := sess.Table(sequence).Cols("last_value", "is_called").Get(&sequenceData); err != nil {
				log.Error("Unable to get last_value and is_called from %s. Error: %v", sequence, err)
				return err
			}
			sequenceMap[sequence] = sequenceData
		}

		// CASCADE causes postgres to drop all the constraints on the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s` CASCADE", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		// CASCADE causes postgres to move all the constraints from the temporary table to the new table
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}

		var indices []string
		sess.Engine().SetSchema("")
		if err := sess.Table("pg_indexes").Cols("indexname").Where("tablename = ? ", tableName).Find(&indices); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
		sess.Engine().SetSchema(schema)

		for _, index := range indices {
			newIndexName := strings.Replace(index, "tmp_recreate__", "", 1)
			if _, err := sess.Exec(fmt.Sprintf("ALTER INDEX `%s` RENAME TO `%s`", index, newIndexName)); err != nil {
				log.Error("Unable to rename %s to %s. Error: %v", index, newIndexName, err)
				return err
			}
		}

		var sequences []string
		sess.Engine().SetSchema("")
		if err := sess.Table("information_schema.sequences").Cols("sequence_name").Where("sequence_name LIKE 'tmp_recreate__' || ? || '_%' AND sequence_catalog = ?", tableName, setting.Database.Name).Find(&sequences); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
		sess.Engine().SetSchema(schema)

		for _, sequence := range sequences {
			newSequenceName := strings.Replace(sequence, "tmp_recreate__", "", 1)
			if _, err := sess.Exec(fmt.Sprintf("ALTER SEQUENCE `%s` RENAME TO `%s`", sequence, newSequenceName)); err != nil {
				log.Error("Unable to rename %s sequence to %s. Error: %v", sequence, newSequenceName, err)
				return err
			}
			val, ok := sequenceMap[newSequenceName]
			if newSequenceName == tableName+"_id_seq" {
				if ok && val.LastValue != 0 {
					if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', %d, %t)", newSequenceName, val.LastValue, val.IsCalled)); err != nil {
						log.Error("Unable to reset %s to %d. Error: %v", newSequenceName, val, err)
						return err
					}
				} else {
					// We're going to try to guess this
					if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', COALESCE((SELECT MAX(id)+1 FROM `%s`), 1), false)", newSequenceName, tableName)); err != nil {
						log.Error("Unable to reset %s. Error: %v", newSequenceName, err)
						return err
					}
				}
			} else if ok {
				if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', %d, %t)", newSequenceName, val.LastValue, val.IsCalled)); err != nil {
					log.Error("Unable to reset %s to %d. Error: %v", newSequenceName, val, err)
					return err
				}
			}
		}
	}
	return nil
}

// WARNING: YOU MUST COMMIT THE SESSION AT THE END
func DropTableColumns(sess *xorm.Session, tableName string, columnNames ...string) (err error) {
	if tableName == "" || len(columnNames) == 0 {
		return nil
	}
	// TODO: This will not work if there are foreign keys

	switch {
	case setting.Database.Type.IsPostgreSQL():
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "DROP COLUMN `" + col + "` CASCADE"
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` %s", tableName, cols)); err != nil {
			return fmt.Errorf("drop table `%s` columns %v: %w", tableName, columnNames, err)
		}
	default:
		log.Fatal("Unrecognized DB")
	}

	return nil
}

// ModifyColumn will modify column's type or other property. SQLITE is not supported
func ModifyColumn(x *xorm.Engine, tableName string, col *schemas.Column) error {
	var indexes map[string]*schemas.Index
	var err error
	// MSSQL have to remove index at first, otherwise alter column will fail
	// ref. https://sqlzealots.com/2018/05/09/error-message-the-index-is-dependent-on-column-alter-table-alter-column-failed-because-one-or-more-objects-access-this-column/
	if x.Dialect().URI().DBType == schemas.MSSQL {
		indexes, err = x.Dialect().GetIndexes(x.DB(), context.Background(), tableName)
		if err != nil {
			return err
		}

		for _, index := range indexes {
			_, err = x.Exec(x.Dialect().DropIndexSQL(tableName, index))
			if err != nil {
				return err
			}
		}
	}

	defer func() {
		for _, index := range indexes {
			_, err = x.Exec(x.Dialect().CreateIndexSQL(tableName, index))
			if err != nil {
				log.Error("Create index %s on table %s failed: %v", index.Name, tableName, err)
			}
		}
	}()

	alterSQL := x.Dialect().ModifyColumnSQL(tableName, col)
	if _, err := x.Exec(alterSQL); err != nil {
		return err
	}
	return nil
}
