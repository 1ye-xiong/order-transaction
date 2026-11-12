package redisx

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"net"
	"testing"
	"time"
)

// commandHook models Redis command results at the client protocol boundary.
type commandHook struct {
	held  bool
	fail  bool
	token string
}

func (h *commandHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, n, a string) (net.Conn, error) { return next(ctx, n, a) }
}
func (h *commandHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *commandHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		if h.fail {
			return errors.New("connection lost")
		}
		switch c := cmd.(type) {
		case *redis.StatusCmd:
			c.SetVal("PONG")
		case *redis.BoolCmd:
			if h.held {
				c.SetVal(false)
			} else {
				h.held = true
				h.token = cmd.Args()[2].(string)
				c.SetVal(true)
			}
		case *redis.Cmd:
			token := cmd.Args()[4].(string)
			if !h.held || token != h.token {
				c.SetVal(int64(0))
			} else {
				if cmd.Args()[1] == releaseScript {
					h.held = false
				}
				c.SetVal(int64(1))
			}
		}
		return nil
	}
}
func TestLockProtocol(t *testing.T) {
	ctx := context.Background()
	c := NewClient(Config{})
	defer c.Close()
	h := &commandHook{}
	c.AddHook(h)
	if e := Ping(ctx, c); e != nil {
		t.Fatal(e)
	}
	locker := NewLocker(c, LockConfig{KeyPrefix: "::p::"})
	l, e := locker.Acquire(ctx, " a ", time.Second)
	if e != nil || l.Key() != "p:a" || len(l.Token()) != 32 {
		t.Fatal(l, e)
	}
	if _, e = locker.Acquire(ctx, "a", time.Second); !errors.Is(e, ErrLockNotAcquired) {
		t.Fatal(e)
	}
	if e = l.Refresh(ctx, time.Second); e != nil {
		t.Fatal(e)
	}
	if e = l.Refresh(ctx, 0); e == nil {
		t.Fatal("ttl")
	}
	if e = l.Release(ctx); e != nil {
		t.Fatal(e)
	}
	if e = l.Release(ctx); !errors.Is(e, ErrLockNotHeld) {
		t.Fatal(e)
	}
	if e = l.Refresh(ctx, time.Second); !errors.Is(e, ErrLockNotHeld) {
		t.Fatal(e)
	}
	h.fail = true
	if _, e = locker.Acquire(ctx, "a", time.Second); e == nil {
		t.Fatal("acquire failure")
	}
	if e = l.Release(ctx); e == nil {
		t.Fatal("release failure")
	}
	if e = l.Refresh(ctx, time.Second); e == nil {
		t.Fatal("refresh failure")
	}
	if (&Locker{cfg: LockConfig{KeyPrefix: "::"}}).fullKey(" a ") != "a" {
		t.Fatal("prefix")
	}
}
