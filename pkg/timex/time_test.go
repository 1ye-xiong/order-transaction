package timex

import (
	"testing"
	"time"
)

func TestRelativeBoundaries(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{{0, "1秒前"}, {-time.Hour, "1秒前"}, {30 * time.Second, "30秒前"}, {2 * time.Minute, "2分钟前"}, {2 * time.Hour, "2小时前"}, {48 * time.Hour, "2天前"}, {60 * 24 * time.Hour, "2月前"}} {
		if got := RelativeTimeInLocation(now.Add(-tc.d), now, time.UTC); got != tc.want {
			t.Fatalf("%v: %s", tc.d, got)
		}
	}
	old := now.Add(-400 * 24 * time.Hour)
	if RelativeTimeAt(old, now) != FormatDateTime(old) {
		t.Fatal("old date")
	}
	if RelativeTimeInLocation(now, now, nil) != "1秒前" || RelativeTime(time.Now()) == "" {
		t.Fatal("default")
	}
}
func TestAgeBirthday(t *testing.T) {
	for _, tc := range []struct {
		birth, now string
		want       int
	}{{"2000-02-29", "2025-02-28", 25}, {"2000-07-01", "2026-06-15", 25}, {"2000-06-20", "2026-06-15", 25}, {"2027-01-01", "2026-06-15", 0}, {"2000-06-01", "2026-06-15", 26}} {
		b, _ := time.Parse("2006-01-02", tc.birth)
		n, _ := time.Parse("2006-01-02", tc.now)
		if AgeAt(b, n) != tc.want {
			t.Fatal(tc)
		}
	}
	if Age(time.Now().AddDate(-5, 0, -1)) != 5 {
		t.Fatal("age")
	}
	if formatRelative(0, "秒") != "1秒前" {
		t.Fatal("clamp")
	}
}
