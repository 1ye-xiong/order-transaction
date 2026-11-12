package repo

import (
	"context"
	"errors"
	"github.com/BwCloudWeGo/bw-cli/internal/user/entity"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"testing"
)

func TestFollowupGormUserRoundTripAndFailures(t *testing.T) {
	db, e := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	if e = AutoMigrate(db); e != nil {
		t.Fatal(e)
	}
	r := NewGormRepository(db, zap.NewNop())
	c := context.Background()
	u, e := entity.NewUser(" Alice ", " Alice ", "salt:hash", "ops")
	if e != nil {
		t.Fatal(e)
	}
	if e = r.Save(c, u); e != nil {
		t.Fatal(e)
	}
	if u.ID == "" {
		t.Fatal("missing id")
	}
	for _, id := range []string{u.ID, "missing"} {
		got, e := r.FindByID(c, id)
		if id == u.ID {
			if e != nil || got.Account != "alice" || got.Role != "ops" {
				t.Fatalf("%+v %v", got, e)
			}
		} else if !errors.Is(e, entity.ErrUserNotFound) {
			t.Fatal(e)
		}
	}
	got, e := r.FindByAccount(c, " ALICE ")
	if e != nil || got.ID != u.ID {
		t.Fatalf("%+v %v", got, e)
	}
	if _, e = r.FindByAccount(c, "missing"); !errors.Is(e, entity.ErrUserNotFound) {
		t.Fatal(e)
	}
	dup := *u
	dup.ID = ""
	if e = r.Save(c, &dup); !errors.Is(e, entity.ErrAccountAlreadyExists) {
		t.Fatal(e)
	}
	u.DisplayName = "Updated"
	if e = r.Save(c, u); e != nil {
		t.Fatal(e)
	}
	bad, cancel := context.WithCancel(c)
	cancel()
	if e = r.Save(bad, u); e == nil {
		t.Fatal("cancel save")
	}
	if _, e = r.FindByID(bad, u.ID); e == nil {
		t.Fatal("cancel read")
	}
	if _, e = r.FindByAccount(bad, u.Account); e == nil {
		t.Fatal("cancel account")
	}
}
