package router

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFrontendStaticRoutesServeBuiltAppAndKeepAPI404(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join("web", "dist", "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{"web/index.html": "legacy", "web/dist/index.html": "react-entry", "web/dist/assets/app.js": "real-bundle"} {
		if err := os.WriteFile(filepath.FromSlash(path), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	r := gin.New()
	registerStaticPages(r)
	for _, c := range []struct {
		path, body string
		code       int
	}{{"/", "react-entry", 200}, {"/orders/view", "react-entry", 200}, {"/assets/app.js", "real-bundle", 200}, {"/legacy", "legacy", 200}, {"/api/v1/missing", "not found", 404}} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("GET", c.path, nil))
		if rec.Code != c.code || !strings.Contains(rec.Body.String(), c.body) {
			t.Fatalf("%s => %d %s", c.path, rec.Code, rec.Body.String())
		}
	}
}
func TestFrontendStaticRoutesFallBackWithoutBuild(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("web", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("web/index.html", []byte("legacy-fallback"), 0644); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	registerStaticPages(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 || rec.Body.String() != "legacy-fallback" {
		t.Fatalf("fallback: %d %s", rec.Code, rec.Body.String())
	}
}
