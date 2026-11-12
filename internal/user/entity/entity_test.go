package entity

import (
	"errors"
	"testing"
)

func TestUserRules(t *testing.T) {
	u, e := NewUser(" A ", " Name ", "hash", "")
	if e != nil || u.Account != "a" || u.Role != RoleBuyer {
		t.Fatal(u, e)
	}
	for _, x := range []struct{ a, n, h, r string }{{"", "n", "h", ""}, {"a", "", "h", ""}, {"a", "n", "", ""}, {"a", "n", "h", "bad"}} {
		if _, e := NewUser(x.a, x.n, x.h, x.r); !errors.Is(e, ErrInvalidUser) {
			t.Fatal(e)
		}
	}
	if NormalizeAccount(" X ") != "x" {
		t.Fatal()
	}
}
