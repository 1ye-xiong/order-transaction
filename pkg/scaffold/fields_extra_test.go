package scaffold

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestColumnMappingVariants(t *testing.T) {
	for _, tc := range []struct {
		db, goType, proto string
		tm                bool
	}{{"boolean", "bool", "bool", false}, {"bigint", "int64", "int64", false}, {"integer", "int64", "int64", false}, {"numeric", "float64", "double", false}, {"float", "float64", "double", false}, {"timestamp", "time.Time", "string", true}, {"blob", "[]byte", "bytes", false}, {"varchar", "string", "string", false}} {
		g, p, tm := mapColumnTypes(tc.db)
		require.Equal(t, tc.goType, g)
		require.Equal(t, tc.proto, p)
		require.Equal(t, tc.tm, tm)
	}
	used := map[string]int{}
	require.Equal(t, "Field", uniqueGoFieldName("", used))
	require.Equal(t, "Field2", uniqueGoFieldName("", used))
	require.Equal(t, "id", fieldVarName("ID"))
	require.Equal(t, "someField", fieldVarName("SomeField"))
	require.Equal(t, "UserId", protoFieldGoName("user_id"))
	require.Equal(t, `bson:"_id"`, bsonTag("ID"))
	require.Equal(t, `bson:"title"`, bsonTag("title"))
	require.Contains(t, gormTag(tableColumn{Name: "id", PrimaryKey: true, AutoIncrement: true}), "autoIncrement")
}
func TestFieldSelectionAndRelations(t *testing.T) {
	fields := []serviceField{{DBName: "name"}, {DBName: "id"}}
	require.Equal(t, "id", choosePrimaryField(fields).DBName)
	require.Equal(t, "name", choosePrimaryField(fields[:1]).DBName)
	require.Equal(t, "id", chooseSortColumn(fields, fields[1]))
	require.False(t, isCreateInput(tableColumn{Name: "updated_at"}))
	require.False(t, isUpdateInput(tableColumn{Name: "deleted_at"}))
	require.False(t, isCreateInput(tableColumn{Name: "id", PrimaryKey: true, HasDefault: true}))
	var d serviceTemplateData
	applyTableColumns(&d, nil)
	require.False(t, d.HasTableFields)
	applyTableColumns(&d, []tableColumn{{Name: "id", DBType: "int", PrimaryKey: true, AutoIncrement: true}, {Name: "created_at", DBType: "timestamp"}, {Name: "value", DBType: "blob"}})
	require.True(t, d.HasTimeFields)
	require.Len(t, d.CreateFields, 1)
	require.Len(t, d.UpdateFields, 1)
	r := TableRelationship{Type: RelationshipOneToMany, FromTable: "parents", FromColumn: "id", ToTable: "children", ToColumn: "parent_id"}
	require.Contains(t, relationJoinSQL(r), "LEFT JOIN parents")
	require.Equal(t, "JoinParentsToChildren", relationMethodName(r))
	r.Type = RelationshipManyToMany
	require.Empty(t, relationJoinSQL(r))
	require.Equal(t, "JoinRelatedTables", relationMethodName(TableRelationship{}))
}
