package service_test

import (
	"context"
	"errors"
	"github.com/BwCloudWeGo/bw-cli/internal/note/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/note/entity"
	svc "github.com/BwCloudWeGo/bw-cli/internal/note/service"
	"testing"
)

type errNoteRepo struct{ err error }

func (r errNoteRepo) Save(context.Context, *entity.Note) error               { return r.err }
func (r errNoteRepo) FindByID(context.Context, string) (*entity.Note, error) { return nil, r.err }
func TestFollowupNoteRepositoryFailures(t *testing.T) {
	ctx := context.Background()
	s := svc.NewService(errNoteRepo{errors.New("db down")})
	if _, e := s.Create(ctx, dto.CreateNoteCommand{AuthorID: "a", Title: "t", Content: "c"}); e == nil {
		t.Fatal("create error swallowed")
	}
	if _, e := s.Get(ctx, "x"); e == nil {
		t.Fatal("get error swallowed")
	}
	if _, e := s.PublishSubmitted(ctx, dto.PublishNoteCommand{AuthorID: "a", Title: "t", Content: "c"}); e == nil {
		t.Fatal("publish error swallowed")
	}
}
