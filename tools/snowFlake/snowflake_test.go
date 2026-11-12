package snowFlake

import "testing"

func TestSnowflake(t *testing.T) {
	if _, e := NewSnowflake(-1); e == nil {
		t.Fatal()
	}
	if _, e := NewSnowflake(1023); e == nil {
		t.Fatal()
	}
	s, e := NewSnowflake(1)
	if e != nil {
		t.Fatal(e)
	}
	a, b := s.NextId(), s.NextId()
	if a == b {
		t.Fatal()
	}
}
