package scaffold

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestModuleAndRewriteFileSelection(t *testing.T) {
	r := t.TempDir()
	p := filepath.Join(r, "go.mod")
	require.NoError(t, os.WriteFile(p, []byte("go 1.25\n"), 0644))
	_, e := readModule(p)
	require.Error(t, e)
	for _, path := range []string{"x.go", "x.mod", "x.md", "x.yaml", "x.yml", "x.proto"} {
		require.True(t, shouldRewrite(path), path)
	}
	require.False(t, shouldRewrite("picture.png"))
	require.NoError(t, os.WriteFile(filepath.Join(r, "x.go"), []byte("package x\n// old.example\n"), 0644))
	require.NoError(t, rewriteModule(r, "old.example", "new.example"))
	b, _ := os.ReadFile(filepath.Join(r, "x.go"))
	require.Contains(t, string(b), "new.example")
	require.Error(t, copyFile(filepath.Join(r, "missing"), filepath.Join(r, "out")))
	require.Error(t, copyFile(filepath.Join(r, "x.go"), r))
	require.Error(t, writeNewFile(p, []byte("x")))
}
