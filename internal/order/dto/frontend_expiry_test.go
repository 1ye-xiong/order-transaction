package dto

import (
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	"testing"
	"time"
)

func TestFrontendExpiryCarriesUnambiguousTimezone(t *testing.T) {
	instant := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	for _, zone := range []*time.Location{time.UTC, time.FixedZone("Shanghai", 8*3600), time.FixedZone("West", -7*3600)} {
		expire := instant.In(zone)
		result := FromOrder(&entity.Order{PayExpireAt: expire})
		parsed, err := time.Parse(time.RFC3339, result.PayExpireAt)
		if err != nil || !parsed.Equal(instant) {
			t.Fatalf("expiry lost timezone: original=%s dto=%s parsed=%s err=%v", expire, result.PayExpireAt, parsed, err)
		}
	}
}
