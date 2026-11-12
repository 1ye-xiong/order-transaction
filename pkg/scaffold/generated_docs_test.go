package scaffold

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestGeneratedDocumentationFunctions(t *testing.T) {
	for _, s := range []string{cleanMakefile(), cleanConfigYAML(), cleanNacosYAML(), cleanREADME("example.com/app"), cleanUsageDoc("example.com/app"), cleanArchitectureDoc(), demoREADME("example.com/app"), demoUsageDoc("example.com/app"), demoArchitectureDoc(), generatedToolkitDoc("example.com/app"), generatedMongoDBDoc("example.com/app"), generatedAlipayDoc(), generatedElasticsearchDoc(), cleanGatewayMain("example.com/app"), cleanRouter("example.com/app"), cleanV1Router()} {
		require.NotEmpty(t, s)
		require.NotEqual(t, "", strings.TrimSpace(s))
	}
}
func TestPlanValidationBranches(t *testing.T) {
	base := GenerationPlan{ServiceName: "x", RootTable: "users", Tables: []string{"users", "orders"}}
	for _, r := range []TableRelationship{{Type: "bad", FromTable: "users", ToTable: "orders", FromColumn: "id", ToColumn: "id"}, {Type: RelationshipOneToMany, FromTable: "bad", ToTable: "orders", FromColumn: "id", ToColumn: "id"}, {Type: RelationshipOneToMany, FromTable: "users", ToTable: "orders", FromColumn: "", ToColumn: "id"}, {Type: RelationshipManyToMany, FromTable: "users", ToTable: "orders", FromColumn: "id", ToColumn: "id"}} {
		p := base
		p.Relationships = []TableRelationship{r}
		require.Error(t, ValidateGenerationPlan(p))
	}
}
