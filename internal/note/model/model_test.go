package model

import "testing"

func TestModel(t *testing.T) {
	if (NoteModel{}).TableName() != "notes" || (NoteDocument{}).MongoCollectionName() != "notes" {
		t.Fatal()
	}
}
