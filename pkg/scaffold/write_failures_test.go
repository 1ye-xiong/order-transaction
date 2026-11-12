package scaffold

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentationWritesPropagatePathFailures(t *testing.T) {
	for _, write := range []struct {
		name string
		fn   func(string, string) error
	}{{"clean", writeCleanDocs}, {"demo", writeDemoDocs}} {
		for _, blocked := range []string{"README.md", "docs/usage.md", "docs/architecture.md", "docs/toolkit.md", "docs/mongodb.md", "docs/alipay.md", "docs/elasticsearch.md"} {
			t.Run(write.name+blocked, func(t *testing.T) {
				r := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(r, "docs"), 0755))
				require.NoError(t, os.MkdirAll(filepath.Join(r, blocked), 0755))
				e := write.fn(r, "example.com/app")
				require.Error(t, e)
			})
		}
	}
}
func TestGatewayAndConfigWritesPropagatePathFailures(t *testing.T) {
	for _, blocked := range []string{"cmd/gateway/main.go", "internal/gateway/router/router.go", "internal/gateway/router/v1.go"} {
		t.Run(blocked, func(t *testing.T) {
			r := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(r, "cmd/gateway"), 0755))
			require.NoError(t, os.MkdirAll(filepath.Join(r, "internal/gateway/router"), 0755))
			require.NoError(t, os.MkdirAll(filepath.Join(r, blocked), 0755))
			require.Error(t, writeCleanGateway(r, "m"))
		})
	}
	r := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(r, "configs/config.yaml"), 0755))
	require.Error(t, writeCleanConfig(r))
	require.NoError(t, os.MkdirAll(filepath.Join(r, "Makefile"), 0755))
	require.Error(t, writeCleanMakefile(r))
	require.NoError(t, os.WriteFile(filepath.Join(r, "file"), []byte("x"), 0644))
	require.Error(t, writeNewFile(filepath.Join(r, "file/x"), []byte("x")))
}
