package entity

import (
	"errors"
	"testing"
)

func TestNoteRules(t *testing.T) {
	n, e := NewNote(" a ", " t ", " c ")
	if e != nil || n.AuthorID != "a" || n.Status != NoteStatusDraft {
		t.Fatal(e)
	}
	n.Publish()
	at := n.PublishedAt
	if n.Status != NoteStatusPublished || at == nil {
		t.Fatal()
	}
	n.Publish()
	var z *Note
	z.Publish()
	for _, x := range [][3]string{{"", "t", "c"}, {"a", "", "c"}, {"a", "t", ""}} {
		if _, e := NewNote(x[0], x[1], x[2]); !errors.Is(e, ErrInvalidNote) {
			t.Fatal(e)
		}
	}
	if NoteStatusFromCode(99) != NoteStatusDraft || NoteStatusPublished.Code() != 2 {
		t.Fatal()
	}
}
