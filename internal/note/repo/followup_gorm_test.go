package repo

import (
	"context"
	"errors"
	"github.com/BwCloudWeGo/bw-cli/internal/note/entity"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"testing"
)

func TestFollowupGormNoteRoundTripAndFailures(t *testing.T) {
	db, e := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	if e = AutoMigrate(db); e != nil {
		t.Fatal(e)
	}
	r := NewGormRepository(db, zap.NewNop())
	c := context.Background()
	n, e := entity.NewNote("a", "title", "content")
	if e != nil {
		t.Fatal(e)
	}
	n.Publish()
	n.NoteType = 2
	n.Permission = 3
	n.Remark = "remark"
	if e = r.Save(c, n); e != nil {
		t.Fatal(e)
	}
	got, e := r.FindByID(c, n.ID)
	if e != nil || got.Title != n.Title || got.Status != entity.NoteStatusPublished || got.PublishedAt == nil || got.Permission != 3 {
		t.Fatalf("roundtrip %+v %v", got, e)
	}
	if _, e = r.FindByID(c, "missing"); !errors.Is(e, entity.ErrNoteNotFound) {
		t.Fatal(e)
	}
	bad, cancel := context.WithCancel(c)
	cancel()
	if e = r.Save(bad, n); e == nil {
		t.Fatal("cancel save")
	}
	if _, e = r.FindByID(bad, n.ID); e == nil {
		t.Fatal("cancel read")
	}
}
