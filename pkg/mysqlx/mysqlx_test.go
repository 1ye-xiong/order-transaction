package mysqlx

import "testing"

func TestConfigAndOpen(t *testing.T) {
	c := DefaultConfig()
	if c.MaxOpenConns == 0 {
		t.Fatal()
	}
	if _, e := Open(Config{}); e == nil {
		t.Fatal()
	}
	if _, e := Open(Config{DSN: "bad"}); e == nil {
		t.Fatal()
	}
}
