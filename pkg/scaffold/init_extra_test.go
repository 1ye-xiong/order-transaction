package scaffold

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestInitValidationAndCopy(t *testing.T) {
	if Init(InitOptions{}) == nil {
		t.Fatal()
	}
	if Init(InitOptions{TargetDir: t.TempDir()}) == nil {
		t.Fatal()
	}
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "go.mod"), []byte("module old.example\n"), 0644)
	os.WriteFile(filepath.Join(src, "x.log"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(src, "data"), 0755)
	os.WriteFile(filepath.Join(src, "data", "x"), []byte("x"), 0644)
	dst := filepath.Join(t.TempDir(), "out")
	if e := Init(InitOptions{SourceDir: src, TargetDir: dst, ModulePath: "new.example"}); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(dst, "x.log")); !os.IsNotExist(e) {
		t.Fatal("log copied")
	}
	b, _ := os.ReadFile(filepath.Join(dst, "go.mod"))
	if string(b) != "module new.example\n" {
		t.Fatal(string(b))
	}
}

func TestGeneratedProjectAddDeleteLifecycle(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "go.mod"), []byte("module example.com/app\n"), 0644))
	dst := filepath.Join(t.TempDir(), "out")
	require.NoError(t, Init(InitOptions{SourceDir: src, TargetDir: dst, ModulePath: "example.com/app"}))
	require.NoError(t, AddService(ServiceOptions{RootDir: dst, Name: "catalog"}))
	require.FileExists(t, filepath.Join(dst, "internal", "catalog", "service", "service.go"))
	require.Error(t, AddService(ServiceOptions{RootDir: dst, Name: "catalog"}))
	require.NoError(t, DeleteService(DeleteServiceOptions{RootDir: dst, Name: "catalog"}))
	require.NoFileExists(t, filepath.Join(dst, "internal", "catalog", "service", "service.go"))
	require.NoError(t, DeleteService(DeleteServiceOptions{RootDir: dst, Name: "catalog"}))
}

func TestInitDemoAndFailures(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "go.mod"), []byte("module old.example\n"), 0644))
	dst := filepath.Join(t.TempDir(), "demo")
	require.NoError(t, Init(InitOptions{SourceDir: src, TargetDir: dst, ModulePath: "new.example", IncludeDemo: true}))
	require.FileExists(t, filepath.Join(dst, "go.mod"))
	require.Error(t, Init(InitOptions{SourceDir: filepath.Join(src, "missing"), TargetDir: filepath.Join(t.TempDir(), "out"), ModulePath: "x"}))
	require.Error(t, copyDir(filepath.Join(src, "go.mod"), t.TempDir()))
	require.Error(t, AddService(ServiceOptions{RootDir: t.TempDir(), Name: "x"}))
	require.Error(t, DeleteService(DeleteServiceOptions{RootDir: t.TempDir(), Name: "x"}))
	require.Error(t, AddService(ServiceOptions{RootDir: dst, Name: "../bad"}))
	require.Error(t, DeleteService(DeleteServiceOptions{RootDir: dst, Name: "../bad"}))
	require.Error(t, AddService(ServiceOptions{RootDir: dst, Name: "x", Port: 65536}))
	require.Error(t, AddService(ServiceOptions{RootDir: dst, Name: "x", TableName: "bad-name"}))
}
func TestHelpers(t *testing.T) {
	if _, e := readModule(filepath.Join(t.TempDir(), "none")); e == nil {
		t.Fatal()
	}
	if !shouldSkip("x.log", fakeEntry{name: "x.log"}) {
		t.Fatal()
	}
}

type fakeEntry struct{ name string }

func (f fakeEntry) Name() string             { return f.name }
func (fakeEntry) IsDir() bool                { return false }
func (fakeEntry) Type() os.FileMode          { return 0 }
func (fakeEntry) Info() (os.FileInfo, error) { return nil, nil }
