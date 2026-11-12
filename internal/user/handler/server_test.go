package handler

import (
	"context"
	"errors"
	u "github.com/BwCloudWeGo/bw-cli/api/gen/user/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/user/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/user/entity"
	"github.com/BwCloudWeGo/bw-cli/internal/user/service"
	"go.uber.org/zap"
	"testing"
)

type repo struct {
	v   *entity.User
	err error
}

func (r *repo) Save(_ context.Context, v *entity.User) error           { v.ID = "1"; r.v = v; return r.err }
func (r *repo) FindByID(context.Context, string) (*entity.User, error) { return r.v, r.err }
func (r *repo) FindByAccount(context.Context, string) (*entity.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.v == nil {
		return nil, entity.ErrUserNotFound
	}
	return r.v, nil
}

type hasher struct{}

func (hasher) Hash(s string) (string, error) { return s, nil }
func (hasher) Verify(a, b string) bool       { return a == b }
func TestRPC(t *testing.T) {
	r := &repo{}
	s := NewServer(service.NewService(r, hasher{}), zap.NewNop())
	ctx := context.Background()
	v, e := s.Register(ctx, &u.RegisterRequest{Account: "a", DisplayName: "n", Password: "p"})
	if e != nil || v.Id != "1" {
		t.Fatal(v, e)
	}
	if _, e = s.Login(ctx, &u.LoginRequest{Account: "a", Password: "p"}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.GetUser(ctx, &u.GetUserRequest{Id: "1"}); e != nil {
		t.Fatal(e)
	}
	r.err = errors.New("db")
	if _, e = s.Register(ctx, &u.RegisterRequest{Account: "a", Password: "p"}); e == nil {
		t.Fatal()
	}
	if _, e = s.Login(ctx, &u.LoginRequest{Account: "a"}); e == nil {
		t.Fatal()
	}
	if _, e = s.GetUser(ctx, &u.GetUserRequest{}); e == nil {
		t.Fatal()
	}
}
func TestMapping(t *testing.T) {
	u := toProto(&dto.UserDTO{ID: "1", Account: "a", DisplayName: "n", Role: "buyer"})
	if u.GetId() != "1" {
		t.Fatal()
	}
	for _, e := range []error{entity.ErrInvalidUser, entity.ErrAccountAlreadyExists, entity.ErrUserNotFound, entity.ErrInvalidCredentials, errors.New("x")} {
		if mapUserError(e) == nil {
			t.Fatal()
		}
	}
}
