package filex

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeBackend struct {
	err error
	key string
}

func (f fakeBackend) Provider() string { return "fake" }
func (f fakeBackend) Bucket() string   { return "b" }
func (f fakeBackend) Put(_ context.Context, r preparedUpload) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return r.Key, nil
}
func TestHelpersAndUpload(t *testing.T) {
	cfg := DefaultConfig()
	if DetectContentType("a.docx") == "" || NewObjectKey("/p/", "a.PDF") == "" {
		t.Fatal()
	}
	for _, r := range []UploadRequest{{Filename: "", Size: 1}, {Filename: "a.exe", Size: 1}, {Filename: "a.pdf", Size: 0}, {Filename: "a.pdf", Size: 1, ContentType: "application/x"}} {
		if ValidateUpload(cfg, r) == nil {
			t.Fatal(r)
		}
	}
	if !contentTypeAllowed([]string{"image/*"}, "image/png") {
		t.Fatal()
	}
	if normalizeContentType(" Text/Plain; charset=utf-8") != "text/plain" {
		t.Fatal()
	}
	u := &uploader{cfg: cfg, backend: fakeBackend{}}
	got, e := u.Upload(context.Background(), UploadRequest{Reader: bytes.NewBufferString("x"), Filename: "a.pdf", Size: 1})
	if e != nil || got.Provider != "fake" {
		t.Fatal(got, e)
	}
	u.backend = fakeBackend{err: errors.New("x")}
	if _, e = u.Upload(context.Background(), UploadRequest{Reader: strings.NewReader("x"), Filename: "a.pdf", Size: 1}); e == nil {
		t.Fatal()
	}
}
func TestBackendValidation(t *testing.T) {
	for _, p := range []string{"minio", "oss", "qiniu", "cos", "bad"} {
		if _, e := newBackend(Config{Provider: p}); e == nil {
			t.Fatal(p)
		}
	}
}
