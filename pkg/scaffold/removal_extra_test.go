package scaffold

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestGatewayClientRemovalRetainsOtherConnections(t *testing.T) {
	r := t.TempDir()
	p := filepath.Join(r, "clients.go")
	d, _ := buildServiceTemplateData("example.com/app", "catalog", 9100)
	require.NoError(t, ensureGatewayClientsFile(p, d))
	b, _ := os.ReadFile(p)
	require.Contains(t, string(b), "catalogConn")
	require.NoError(t, removeGatewayClient(p, d))
	b, _ = os.ReadFile(p)
	require.NotContains(t, string(b), "catalogConn")
	require.NotContains(t, string(b), "CatalogServiceClient")
	require.Equal(t, "conns: []*grpc.ClientConn{otherConn}", removeGatewayConnFromList("conns: []*grpc.ClientConn{catalogConn, otherConn,}", "catalogConn"))
	require.NotContains(t, removeGatewayConnCloseLines("\tcatalogConn.Close()\nkeep\n", "catalogConn"), "Close")
}
func TestDirectoryCleanupPreservesKeepAndRemovesRoutes(t *testing.T) {
	r := t.TempDir()
	for _, d := range []string{"keep", "remove"} {
		require.NoError(t, os.MkdirAll(filepath.Join(r, d), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(r, d, "x"), []byte("x"), 0644))
	}
	require.NoError(t, removeChildrenExcept(r, "keep"))
	require.DirExists(t, filepath.Join(r, "keep"))
	require.NoDirExists(t, filepath.Join(r, "remove"))
	require.NoError(t, os.WriteFile(filepath.Join(r, "x_routes.go"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(r, "keep.go"), []byte("x"), 0644))
	require.NoError(t, removeGeneratedGatewayRoutes(r))
	require.NoFileExists(t, filepath.Join(r, "x_routes.go"))
	require.FileExists(t, filepath.Join(r, "keep.go"))
	require.NoError(t, removeChildrenExcept(filepath.Join(r, "missing")))
	require.NoError(t, removeGeneratedGatewayRoutes(filepath.Join(r, "missing")))
}
func TestNacosFlagsAndCRUDValidation(t *testing.T) {
	r := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(r, "configs"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(r, "configs/nacos.yaml"), []byte("enabled: true\ndata_id: custom.yaml\n"), 0644))
	enabled, id := nacosEnabled(r)
	require.True(t, enabled)
	require.Equal(t, "custom.yaml", id)
	require.Error(t, validateDefaultCRUDColumns("a", nil))
	require.NoError(t, validateDefaultCRUDColumns("a", []tableColumn{{Name: "id", PrimaryKey: true}}))
	require.Empty(t, lowerFirst(""))
	require.Equal(t, "package x\nimport (\n)\n", removeImport("package x\nimport (\n\t\"fmt\"\n)\n", "\"fmt\""))
}
