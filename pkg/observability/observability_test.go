package observability

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"testing"
)

func TestRegister(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	Register("order", zap.New(core))
	if logs.Len() != 1 || logs.All()[0].ContextMap()["service"] != "order" {
		t.Fatal(logs.All())
	}
}
