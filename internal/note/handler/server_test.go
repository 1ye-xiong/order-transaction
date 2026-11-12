package handler

import (
	"context"
	"errors"
	n "github.com/BwCloudWeGo/bw-cli/api/gen/note/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/note/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/note/entity"
	"github.com/BwCloudWeGo/bw-cli/internal/note/service"
	"go.uber.org/zap"
	"testing"
)

type repo struct {
	v   *entity.Note
	err error
}

func (r *repo) Save(_ context.Context, v *entity.Note) error           { r.v = v; return r.err }
func (r *repo) FindByID(context.Context, string) (*entity.Note, error) { return r.v, r.err }
func TestRPC(t *testing.T) {
	r := &repo{}
	s := NewServer(service.NewService(r), zap.NewNop())
	ctx := context.Background()
	v, e := s.CreateNote(ctx, &n.CreateNoteRequest{AuthorId: "a", Title: "t", Content: "c"})
	if e != nil || v.Title != "t" {
		t.Fatal(v, e)
	}
	if _, e = s.GetNote(ctx, &n.GetNoteRequest{Id: v.Id}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.PublishNote(ctx, &n.PublishNoteRequest{AuthorId: "a", Title: "t", Content: "c", Status: 2}); e != nil {
		t.Fatal(e)
	}
	r.err = entity.ErrNoteNotFound
	if _, e = s.GetNote(ctx, &n.GetNoteRequest{}); e == nil {
		t.Fatal()
	}
	if _, e = s.CreateNote(ctx, &n.CreateNoteRequest{}); e == nil {
		t.Fatal()
	}
	if _, e = s.PublishNote(ctx, &n.PublishNoteRequest{}); e == nil {
		t.Fatal()
	}
}
func TestMapping(t *testing.T) {
	n := toProto(&dto.NoteDTO{ID: "1", AuthorID: "a", Title: "t", Status: entity.NoteStatusDraft})
	if n.GetId() != "1" {
		t.Fatal()
	}
	for _, e := range []error{entity.ErrInvalidNote, entity.ErrNoteNotFound, errors.New("x")} {
		if mapNoteError(e) == nil {
			t.Fatal()
		}
	}
}
