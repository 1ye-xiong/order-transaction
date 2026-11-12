package middleware

import (
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestJWTTokenAndAuthBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	j := NewJWT(JWTConfig{Secret: "test-secret", Issuer: "test", ExpireSeconds: 60})
	token, err := j.GenerateToken(JWTClaims{UserID: "u1", Role: "ops"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewJWT(JWTConfig{}).GenerateToken(JWTClaims{}); err == nil {
		t.Fatal("missing secret should fail")
	}
	for _, tc := range []struct {
		name, auth string
		want       int
	}{
		{"missing", "", http.StatusUnauthorized}, {"malformed", "Bearer bad", http.StatusUnauthorized},
		{"wrong scheme", "Basic " + token, http.StatusUnauthorized}, {"valid", "Bearer " + token, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/", j.Auth(), func(c *gin.Context) { c.JSON(http.StatusOK, ClaimsFromContext(c)) })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tc.auth)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestInvalidSignedTokens(t *testing.T) {
	cfg := JWTConfig{Secret: "secret", Issuer: "issuer"}
	for _, tc := range []struct {
		name    string
		method  jwt.SigningMethod
		secret  any
		issuer  string
		expires time.Time
	}{{"expired", jwt.SigningMethodHS256, []byte("secret"), "issuer", time.Now().Add(-time.Hour)}, {"wrong issuer", jwt.SigningMethodHS256, []byte("secret"), "other", time.Now().Add(time.Hour)}, {"wrong signature", jwt.SigningMethodHS256, []byte("wrong"), "issuer", time.Now().Add(time.Hour)}, {"none", jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, "issuer", time.Now().Add(time.Hour)}} {
		t.Run(tc.name, func(t *testing.T) {
			token, e := jwt.NewWithClaims(tc.method, registeredJWTClaims{RegisteredClaims: jwt.RegisteredClaims{Issuer: tc.issuer, ExpiresAt: jwt.NewNumericDate(tc.expires)}}).SignedString(tc.secret)
			if e != nil {
				t.Fatal(e)
			}
			r := gin.New()
			r.GET("/", JWTAuth(cfg), func(c *gin.Context) { c.Status(200) })
			q := httptest.NewRequest("GET", "/", nil)
			q.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, q)
			if w.Code != 401 {
				t.Fatal(w.Code)
			}
		})
	}
	r := gin.New()
	r.GET("/", JWTAuth(JWTConfig{}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	if _, e := GenerateToken(cfg, JWTClaims{UserID: "u"}); e != nil {
		t.Fatal(e)
	}
}

func TestDefaultCORSAndLoggerFields(t *testing.T) {
	r := gin.New()
	r.Use(CORS(CORSConfig{}), RequestLogger(zap.NewNop()))
	r.GET("/", func(c *gin.Context) { c.Status(204) })
	q := httptest.NewRequest("GET", "/", nil)
	q.Header.Set("Origin", "https://example.org")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, q)
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal(w.Header())
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if requestID(c) != "" || errorCode(c) != "" {
		t.Fatal("empty")
	}
	c.Set("error_code", 42)
	c.Set("request_id", 42)
	if requestID(c) != "" || errorCode(c) != "" {
		t.Fatal("bad type")
	}
}

func TestRequestLoggerAndClaims(t *testing.T) {
	r := gin.New()
	r.Use(RequestID(), RequestLogger(zap.NewNop()))
	r.GET("/x", func(c *gin.Context) { c.Set("error_code", "E"); c.Status(204) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set(HeaderRequestID, "fixed")
	r.ServeHTTP(w, req)
	if w.Header().Get(HeaderRequestID) != "fixed" {
		t.Fatal()
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if ClaimsFromContext(c).UserID != "" {
		t.Fatal()
	}
	c.Set(claimsContextKey, "bad")
	if ClaimsFromContext(c).UserID != "" {
		t.Fatal()
	}
}

func TestCORSAndRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(CORSConfig{AllowOrigins: []string{"https://ok.example"}, AllowCredentials: true}), RequestID())
	r.Any("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, tc := range []struct {
		origin, method string
		want           int
	}{
		{"https://ok.example", http.MethodOptions, http.StatusNoContent}, {"https://bad.example", http.MethodGet, http.StatusForbidden}, {"", http.MethodGet, http.StatusNoContent},
	} {
		t.Run(tc.origin+tc.method, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/", nil)
			req.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status=%d", w.Code)
			}
			if tc.origin == "" && w.Header().Get(HeaderRequestID) == "" {
				t.Fatal("request id missing")
			}
		})
	}
}
