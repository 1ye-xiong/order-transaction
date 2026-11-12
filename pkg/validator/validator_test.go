package validator

import "testing"

func TestRequired(t *testing.T) {
	if !Required(" a ", "b") || Required("a", "  ") || !Required() {
		t.Fatal("required semantics")
	}
}
