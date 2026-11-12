package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectProtoFilesReturnsSlashSeparatedRelativePaths(t *testing.T) {
	root := t.TempDir()
	protoRoot := filepath.Join(root, "api", "proto")
	require.NoError(t, os.MkdirAll(filepath.Join(protoRoot, "user", "v1"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(protoRoot, "note", "v1"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(protoRoot, "user", "v1", "user.proto"), []byte("syntax = \"proto3\";"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(protoRoot, "note", "v1", "content.proto"), []byte("syntax = \"proto3\";"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(protoRoot, "README.md"), []byte("skip"), 0o644))

	files, err := collectProtoFiles(protoRoot)

	require.NoError(t, err)
	require.Equal(t, []string{"note/v1/content.proto", "user/v1/user.proto"}, files)
}

func TestCollectProtoFilesAllowsMissingProtoDirectory(t *testing.T) {
	files, err := collectProtoFiles(filepath.Join(t.TempDir(), "api", "proto"))

	require.NoError(t, err)
	require.Empty(t, files)
}

func TestPrependPathUsesCurrentOSPathSeparator(t *testing.T) {
	got := prependPath([]string{"Path=base"}, "first", "second")

	require.Len(t, got, 1)
	require.Equal(t, "Path=first"+string(os.PathListSeparator)+"second"+string(os.PathListSeparator)+"base", got[0])
}

func TestPrependPathAddsPathWhenMissing(t *testing.T) {
	got := prependPath([]string{"OTHER=value"}, "bin")

	require.Len(t, got, 2)
	require.True(t, strings.HasPrefix(got[1], "PATH=bin"))
}

func TestProtoHelpersAndConfig(t *testing.T) {
	t.Setenv("PROTOC", " custom ")
	t.Setenv("PROTO_PATH", " proto ")
	t.Setenv("PROTO_OUT", " out ")
	c := protoConfigFromEnv()
	require.Equal(t, "custom", c.Protoc)
	require.Equal(t, "proto", c.ProtoPath)
	require.Equal(t, "out", c.ProtoOut)
	t.Setenv("PROTOC", "")
	require.Equal(t, "protoc", protoConfigFromEnv().Protoc)
	require.Equal(t, "fallback", getenvDefault("MISSING_ENV_AUDIT", "fallback"))
	require.Equal(t, []string{"a", "b"}, cleanPathEntries([]string{" a ", "", "b"}))
}
func TestCollectProtoFilesErrorsOnFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "notdir")
	require.NoError(t, os.WriteFile(p, []byte("x"), 0644))
	_, err := collectProtoFiles(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a directory")
}
func TestPrependPathHandlesMalformedAndExistingCaseInsensitive(t *testing.T) {
	got := prependPath([]string{"bad", "path=base"}, " ", "new")
	require.Equal(t, "path=new"+string(os.PathListSeparator)+"base", got[1])
	got = prependPath([]string{"Path=base"})
	require.Equal(t, []string{"Path=base"}, got)
}
func TestWithGoPluginPathPreservesEnvironment(t *testing.T) {
	got := withGoPluginPath([]string{"OTHER=x"})
	require.Contains(t, strings.ToLower(strings.Join(got, ";")), "path=")
}
