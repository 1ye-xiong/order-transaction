package postgresx

import "testing"

func TestConfigAndOpen(t *testing.T) {
	c := DefaultConfig()
	if c.MaxOpenConns == 0 {
		t.Fatal()
	}
	if _, e := Open(Config{}); e == nil {
		t.Fatal()
	}
	if _, e := Open(Config{DSN: "postgres://bad:bad@127.0.0.1:1/bad?connect_timeout=1"}); e == nil {
		t.Fatal()
	}
}
