package redisx

import (
	"context"
	"github.com/redis/go-redis/v9"
	"testing"
	"time"
)

func TestConfigAndNilLocker(t *testing.T) {
	c := (Config{}).Normalize()
	if c.Addr == "" || c.PoolSize == 0 {
		t.Fatal()
	}
	cl := NewClient(Config{})
	if cl == nil {
		t.Fatal()
	}
	l := NewLocker(nil, LockConfig{})
	if _, ok, e := l.TryAcquire(context.Background(), "x", time.Second); e == nil || ok {
		t.Fatal()
	}
	if _, e := l.Acquire(context.Background(), "x", time.Second); e == nil {
		t.Fatal()
	}
	var z *Lock
	if z.Key() != "" || z.Token() != "" || z.Release(context.Background()) != ErrLockNotHeld {
		t.Fatal()
	}
	_ = redis.Nil
}
