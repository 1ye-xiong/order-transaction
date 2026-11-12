package entity

import "testing"

func TestAllStatusCodes(t *testing.T) {
	for _, x := range []struct {
		code   int32
		status NoteStatus
	}{{1, NoteStatusDraft}, {2, NoteStatusPublished}} {
		if NoteStatusFromCode(x.code) != x.status || x.status.Code() != x.code {
			t.Fatal(x)
		}
	}
	if NoteStatus("unknown").Code() != 0 {
		t.Fatal()
	}
}
