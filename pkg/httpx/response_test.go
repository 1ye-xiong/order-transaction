package httpx

import (
	"encoding/json"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("request_id", "r")
	OK(c, map[string]int{"a": 1})
	var body map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &body); e != nil || w.Code != 200 || body["request_id"] != "r" || body["data"].(map[string]any)["a"] != float64(1) {
		t.Fatal(w.Body.String(), e)
	}
	if RequestID(c) != "r" {
		t.Fatal()
	}
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	Created(c2, "x")
	if w2.Code != 201 {
		t.Fatal(w2.Code)
	}
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	Error(c3, assertErr{})
	if w3.Code != 500 || json.Unmarshal(w3.Body.Bytes(), &body) != nil || body["error"].(map[string]any)["message"] != "internal error" {
		t.Fatal(w3.Body.String())
	}
	w5 := httptest.NewRecorder()
	c5, _ := gin.CreateTestContext(w5)
	Error(c5, apperrors.NotFound("nf", "missing"))
	if w5.Code != 404 || c5.GetString("error_code") != "nf" {
		t.Fatal(w5.Body.String())
	}
	c4, _ := gin.CreateTestContext(httptest.NewRecorder())
	c4.Set("request_id", 123)
	if RequestID(c4) != "" {
		t.Fatal()
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "bad" }
