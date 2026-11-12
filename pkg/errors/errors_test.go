package errors

import (
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

func TestMappingsAndRoundTrip(t *testing.T) {
	kinds := []Kind{KindInvalidArgument, KindUnauthorized, KindForbidden, KindNotFound, KindConflict, KindInternal}
	for _, k := range kinds {
		e := New(k, "x", "m")
		if e.Error() == "" || HTTPStatus(e) == 0 || GRPCCode(e) == codes.OK {
			t.Fatal(k)
		}
		r := FromGRPC(ToGRPC(e))
		if r.Code != "x" || r.Message != "m" {
			t.Fatalf("%+v", r)
		}
	}
	if ToGRPC(nil) != nil || FromGRPC(nil) != nil {
		t.Fatal("nil")
	}
	if _, ok := As(errors.New("x")); ok {
		t.Fatal("as")
	}
	if HTTPStatus(errors.New("x")) != 500 {
		t.Fatal("status")
	}
	wrapped := Wrap(KindInternal, "c", "m", errors.New("cause"))
	if !errors.Is(wrapped, wrapped.Cause) || wrapped.Unwrap() == nil {
		t.Fatal("unwrap")
	}
	if FromGRPC(status.Error(codes.Unknown, "plain")).Code != "internal_error" {
		t.Fatal("plain")
	}
}

func TestConstructorsAndNil(t *testing.T) {
	var n *AppError
	if n.Error() != "" || n.Unwrap() != nil {
		t.Fatal("nil receiver")
	}
	for _, e := range []*AppError{InvalidArgument("a", "m"), Unauthorized("b", "m"), NotFound("c", "m"), Conflict("d", "m"), Internal("e", "m")} {
		if e.Message != "m" {
			t.Fatal(e)
		}
	}
	cause := errors.New("root")
	e := Wrap(KindInternal, "x", "message", cause)
	if e.Error() != "x: message: root" {
		t.Fatal(e)
	}
	if GRPCCode(cause) != codes.Internal || FromGRPC(cause).Kind != KindInternal || FromGRPC(ToGRPC(cause)).Code != "internal_error" {
		t.Fatal("unknown masking")
	}
	if FromGRPC(status.Error(codes.AlreadyExists, "duplicate")).Kind != KindConflict {
		t.Fatal("duplicate")
	}
}
