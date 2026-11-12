package dto

import (
	"github.com/BwCloudWeGo/bw-cli/internal/note/entity"
	"testing"
)

func TestFromNote(t *testing.T) {
	if FromNote(&entity.Note{ID: "1", AuthorID: "a", Status: entity.NoteStatusDraft}).ID != "1" {
		t.Fatal()
	}
}
