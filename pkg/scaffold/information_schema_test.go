package scaffold

import (
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
)

// A local metadata fixture exercises result mapping without external DB credentials.
func TestInformationSchemaMetadataMapping(t *testing.T) {
	db, e := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, e)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	for _, q := range []string{`attach database ':memory:' as information_schema`, `create table information_schema.columns(table_schema text,table_name text,column_name text,data_type text,is_nullable text,column_key text,extra text,column_default text,ordinal_position int)`, `create table information_schema.key_column_usage(table_schema text,table_name text,column_name text,referenced_table_name text,referenced_column_name text,constraint_schema text,constraint_name text)`, `create table information_schema.table_constraints(table_schema text,constraint_schema text,constraint_name text,constraint_type text)`, `create table information_schema.constraint_column_usage(constraint_schema text,constraint_name text,table_name text,column_name text)`, `insert into information_schema.columns values ('public','items','id','bigint','NO','PRI','auto_increment','1',1)`, `insert into information_schema.key_column_usage values ('public','items','id','parents','id','public','pk')`, `insert into information_schema.table_constraints values ('public','public','pk','PRIMARY KEY')`} {
		require.NoError(t, db.Exec(q).Error)
	}
	cols, e := inspectMySQLColumns(db, "items", "public")
	require.NoError(t, e)
	require.Len(t, cols, 1)
	require.True(t, cols[0].PrimaryKey)
	require.True(t, cols[0].AutoIncrement)
	require.True(t, cols[0].HasDefault)
	cols, e = inspectPostgreSQLColumns(db, "items", "")
	require.NoError(t, e)
	require.True(t, cols[0].PrimaryKey)
	snap, e := inspectInformationSchema(db, "mysql", "public", true)
	require.NoError(t, e)
	require.Len(t, snap.Tables, 1)
	require.Len(t, snap.ForeignKeys, 1)
	require.Equal(t, "parents", snap.ForeignKeys[0].ToTable)
	snap, e = inspectInformationSchema(db, "postgres", "", false)
	require.NoError(t, e)
	require.Equal(t, "public", snap.Schema)
	require.Len(t, snap.Tables, 1)
	_, e = inspectMySQLColumns(db, "missing", "public")
	require.Error(t, e)
}
