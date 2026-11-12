package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGateOverallAndEveryCorePackage(t *testing.T) {
	cases := []struct {
		name, profile string
		want          int
	}{
		{"pass", "mode: atomic\nm/internal/order/service/a.go:1.1,1.2 100 1\nm/pkg/config/a.go:1.1,1.2 100 1\nm/api/gen/a.go:1.1,1.2 10000 0\n", 0},
		{"overall fails", "mode: atomic\nm/pkg/config/a.go:1.1,1.2 100 0\n", 1},
		{"single core fails", "mode: atomic\nm/internal/order/service/a.go:1.1,1.2 1 0\nm/pkg/config/a.go:1.1,1.2 999 1\n", 1},
		{"empty", "mode: atomic\n", 1},
		{"bad statement", "mode: atomic\nm/pkg/a.go:1.1,1.2 nope 1\n", 2},
		{"bad hits", "mode: atomic\nm/pkg/a.go:1.1,1.2 1 nope\n", 2},
		{"scanner error", strings.Repeat("x", 70000), 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "coverage.out")
			if err := os.WriteFile(p, []byte(tc.profile), 0600); err != nil {
				t.Fatal(err)
			}
			var out, errout bytes.Buffer
			if got := run([]string{p}, &out, &errout); got != tc.want {
				t.Fatalf("exit=%d want %d: %s %s", got, tc.want, out.String(), errout.String())
			}
		})
	}
	var out bytes.Buffer
	if run(nil, &out, &out) != 2 {
		t.Fatal("usage")
	}
	if run([]string{filepath.Join(t.TempDir(), "missing")}, &out, &out) != 2 {
		t.Fatal("missing")
	}
}
