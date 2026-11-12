package dto

import (
	"github.com/BwCloudWeGo/bw-cli/internal/user/entity"
	"testing"
)

func TestFromUser(t *testing.T) {
	if FromUser(&entity.User{ID: "1", Account: "a", Role: "buyer"}).ID != "1" {
		t.Fatal()
	}
}
