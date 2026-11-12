package service_test

import (
	"context"
	"errors"
	"github.com/BwCloudWeGo/bw-cli/internal/user/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/user/entity"
	svc "github.com/BwCloudWeGo/bw-cli/internal/user/service"
	"testing"
)

type errUserRepo struct{ err error }

func (r errUserRepo) Save(context.Context, *entity.User) error                    { return r.err }
func (r errUserRepo) FindByID(context.Context, string) (*entity.User, error)      { return nil, r.err }
func (r errUserRepo) FindByAccount(context.Context, string) (*entity.User, error) { return nil, r.err }

type errHasher struct{ err error }

func (errHasher) Hash(string) (string, error) { return "", errors.New("hash failed") }
func (errHasher) Verify(string, string) bool  { return false }
func TestFollowupUserFailurePropagation(t *testing.T) {
	ctx := context.Background()
	mem := newMemoryUserRepo()
	valid := newTestService(mem)
	u, e := valid.Register(ctx, dto.RegisterCommand{Account: "get", DisplayName: "Get", Password: "pw"})
	if e != nil {
		t.Fatal(e)
	}
	if got, e := valid.GetUser(ctx, u.ID); e != nil || got.ID != u.ID {
		t.Fatalf("get %v %v", got, e)
	}
	if _, e := valid.Register(ctx, dto.RegisterCommand{Account: "blank", Password: " "}); !errors.Is(e, entity.ErrInvalidUser) {
		t.Fatal(e)
	}
	if _, e := valid.Register(ctx, dto.RegisterCommand{Account: "invalid", Password: "pw", Role: "root"}); !errors.Is(e, entity.ErrInvalidUser) {
		t.Fatal(e)
	}
	if _, e := valid.Login(ctx, dto.LoginCommand{Account: "unknown"}); !errors.Is(e, entity.ErrInvalidCredentials) {
		t.Fatal(e)
	}
	s := svc.NewService(errUserRepo{errors.New("db down")}, plainHasher{})
	if _, e := s.Register(ctx, dto.RegisterCommand{Account: "a", Password: "pw"}); e == nil {
		t.Fatal("lookup error swallowed")
	}
	s = svc.NewService(newMemoryUserRepo(), errHasher{})
	if _, e := s.Register(ctx, dto.RegisterCommand{Account: "new", Password: "pw"}); e == nil {
		t.Fatal("hash failure swallowed")
	}
	s = svc.NewService(errUserRepo{errors.New("db down")}, plainHasher{})
	if _, e := s.Login(ctx, dto.LoginCommand{Account: "x"}); e == nil {
		t.Fatal("login query failure swallowed")
	}
	if _, e := s.GetUser(ctx, "x"); e == nil {
		t.Fatal("user query failure swallowed")
	}
}
