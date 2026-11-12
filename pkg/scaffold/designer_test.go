package scaffold

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDesignerPlanEndpointSavesPlan(t *testing.T) {
	root := t.TempDir()
	mux := newDesignerMux(DesignerOptions{RootDir: root})
	body := strings.NewReader(`{"service_name":"product","root_table":"product_spu","tables":["product_spu"],"relationships":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/plan", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.FileExists(t, filepath.Join(root, "scaffold-plans", "product.json"))
}

func TestDesignerListenFailureAndAPIValidation(t *testing.T) {
	r := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(r, "go.mod"), []byte("module example.com/app\n"), 0644))
	l, e := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, e)
	defer l.Close()
	require.Error(t, StartDesigner(DesignerOptions{RootDir: r, Addr: l.Addr().String()}))
	require.Error(t, StartDesigner(DesignerOptions{RootDir: filepath.Join(r, "missing")}))
	mux := newDesignerMux(DesignerOptions{RootDir: r})
	for _, tc := range []struct {
		path, body string
		code       int
	}{{"/api/schema", `{"driver":"sqlite","dsn":":memory:"}`, 200}, {"/api/schema", `{"driver":"bad","dsn":"bad"}`, 400}, {"/api/generate", `{"plan":{"service_name":"../bad"}}`, 400}, {"/api/generate", `{"plan":{"service_name":"catalog","root_table":"items","tables":["items"]}}`, 200}} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body)))
		require.Equal(t, tc.code, w.Code, w.Body.String())
	}
	require.FileExists(t, filepath.Join(r, "internal/catalog/service/service.go"))
}

func TestDesignerEndpointsRejectInvalidRequests(t *testing.T) {
	root := t.TempDir()
	mux := newDesignerMux(DesignerOptions{RootDir: root})
	for _, tc := range []struct {
		path, method, body string
		code               int
	}{{"/api/plan", "GET", "", 405}, {"/api/plan", "POST", "{", 400}, {"/api/schema", "GET", "", 405}, {"/api/schema", "POST", "{", 400}, {"/api/generate", "GET", "", 405}, {"/api/generate", "POST", "{", 400}, {"/missing", "GET", "", 404}} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		require.Equal(t, tc.code, w.Code, tc.path)
	}
}

func TestDesignerRootAndPlanValidation(t *testing.T) {
	mux := newDesignerMux(DesignerOptions{RootDir: t.TempDir()})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	require.Equal(t, 200, w.Code)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/plan", strings.NewReader(`{"service_name":"../bad"}`)))
	require.Equal(t, 400, w.Code)
}
