package scaffold

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestGatewayFullServiceLifecycle(t *testing.T) {
	r := t.TempDir()
	for _, d := range []string{"cmd/gateway", "internal/gateway/router", "configs"} {
		require.NoError(t, os.MkdirAll(filepath.Join(r, d), 0755))
	}
	for p, s := range map[string]string{"go.mod": "module example.com/app\n", "Makefile": ".PHONY: test\ntest:\n\tgo test ./...\n", "configs/config.yaml": "services:\n\ndatabase:\n  driver: sqlite\n", "cmd/gateway/main.go": cleanGatewayMain("example.com/app"), "internal/gateway/router/router.go": cleanRouter("example.com/app"), "internal/gateway/router/v1.go": cleanV1Router()} {
		require.NoError(t, os.WriteFile(filepath.Join(r, p), []byte(s), 0644))
	}
	require.NoError(t, AddService(ServiceOptions{RootDir: r, Name: "catalog"}))
	for _, p := range []string{"internal/gateway/request/catalog_request.go", "internal/gateway/handler/catalog_handler.go", "internal/gateway/router/catalog_routes.go"} {
		require.FileExists(t, filepath.Join(r, p))
	}
	b, _ := os.ReadFile(filepath.Join(r, "Makefile"))
	require.Contains(t, string(b), "run-catalog:")
	require.NoError(t, DeleteService(DeleteServiceOptions{RootDir: r, Name: "catalog"}))
	b, _ = os.ReadFile(filepath.Join(r, "internal/gateway/router/v1.go"))
	require.NotContains(t, string(b), "registerCatalogRoutes")
	require.Contains(t, string(b), "_ = v1")
	require.False(t, hasGatewayRouteRegistration(string(b)))
	require.NoError(t, ensureGatewayCommonFile(filepath.Join(r, "internal/gateway/handler/common.go"), serviceTemplateData{}))
}
