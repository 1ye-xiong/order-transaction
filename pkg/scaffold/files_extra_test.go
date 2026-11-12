package scaffold

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanAndDemoWriters(t *testing.T) {
	r := t.TempDir()
	for _, d := range []string{"cmd/gateway", "internal/gateway/router", "configs", "docs"} {
		require.NoError(t, os.MkdirAll(filepath.Join(r, d), 0755))
	}
	for _, p := range []string{"Makefile", "README.md", "configs/config.yaml"} {
		require.NoError(t, os.WriteFile(filepath.Join(r, p), []byte("old"), 0644))
	}
	require.NoError(t, writeCleanGateway(r, "m"))
	require.NoError(t, writeCleanMakefile(r))
	require.NoError(t, writeCleanConfig(r))
	require.NoError(t, writeCleanDocs(r, "m"))
	for _, p := range []string{"README.md", "docs/usage.md", "docs/architecture.md", "docs/toolkit.md", "docs/mongodb.md", "docs/alipay.md", "docs/elasticsearch.md", "configs/nacos.yaml"} {
		require.FileExists(t, filepath.Join(r, p))
	}
	require.NoError(t, writeDemoDocs(r, "m"))
}
func TestMakeAndConfigRemoval(t *testing.T) {
	r := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(r, "configs"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(r, "Makefile"), []byte(".PHONY: run-a run-b\nrun-a:\n\tgo run ./cmd/a\nrun-b:\n\tgo run ./cmd/b\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(r, "configs/config.yaml"), []byte("services:\n  a:\n    port: 1\n  b:\n    port: 2\n\ndatabase:\n  driver: sqlite\n"), 0644))
	require.NoError(t, removeServiceMakeTarget(r, "a"))
	require.NoError(t, removeServiceConfig(r, "a"))
	b, _ := os.ReadFile(filepath.Join(r, "Makefile"))
	require.NotContains(t, string(b), "run-a:")
	c, _ := os.ReadFile(filepath.Join(r, "configs/config.yaml"))
	require.NotContains(t, string(c), "  a:")
	require.Equal(t, ".PHONY: run-b", removeMakePhonyTarget(".PHONY: run-a run-b", "run-a"))
}
