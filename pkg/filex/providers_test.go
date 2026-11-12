package filex

import (
	"context"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"hash/crc64"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestOSSAndMinIOHTTP(t *testing.T) {
	for _, provider := range []string{"oss", "minio"} {
		for _, fail := range []bool{false, true} {
			t.Run(provider+strconv.FormatBool(fail), func(t *testing.T) {
				calls := 0
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != "PUT" {
						t.Error(r.Method)
					}
					if fail {
						w.WriteHeader(400)
						return
					}
					w.Header().Set("ETag", `"etag"`)
				}))
				defer srv.Close()
				var b backend
				var e error
				if provider == "oss" {
					client, err := oss.New(srv.URL, "id", "key", oss.ForcePathStyle(true))
					if err != nil {
						t.Fatal(err)
					}
					bucket, err := client.Bucket("bucket")
					if err != nil {
						t.Fatal(err)
					}
					b = &ossBackend{bucket: bucket, cfg: OSSConfig{Bucket: "bucket"}}
				} else {
					b, e = newMinIOBackend(Config{MinIO: MinIOConfig{Endpoint: strings.TrimPrefix(srv.URL, "http://"), AccessKeyID: "id", SecretAccessKey: "key", Bucket: "bucket", Region: "us-east-1"}})
					if e != nil {
						t.Fatal(e)
					}
				}
				if b.Provider() != provider || b.Bucket() != "bucket" {
					t.Fatal("provider")
				}
				meta := map[string]string{"owner": "alice"}
				if provider == "oss" {
					meta[" "] = "bad"
				}
				_, e = b.Put(context.Background(), preparedUpload{UploadRequest: UploadRequest{Reader: strings.NewReader("pdf"), Size: 3, Metadata: meta}, Key: "a.pdf", Content: "application/pdf"})
				if fail && e == nil {
					t.Fatal("failure accepted")
				}
				if !fail && e != nil {
					t.Fatal(e)
				}
				if calls == 0 {
					t.Fatal("no HTTP call")
				}
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if _, e = b.Put(ctx, preparedUpload{UploadRequest: UploadRequest{Reader: strings.NewReader("x"), Size: 1}, Key: "a.pdf"}); e == nil {
					t.Fatal("cancel accepted")
				}
			})
		}
	}
}

func TestCOSUploadHTTP(t *testing.T) {
	for _, fail := range []bool{false, true} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			data, _ := io.ReadAll(r.Body)
			if string(data) != "pdf" || r.Header.Get("X-Cos-Meta-Owner") != "alice" {
				t.Error("wrong body or metadata")
			}
			if fail {
				w.WriteHeader(400)
				return
			}
			w.Header().Set("ETag", `"tag"`)
			w.Header().Set("x-cos-hash-crc64ecma", strconv.FormatUint(crc64.Checksum([]byte("pdf"), crc64.MakeTable(crc64.ECMA)), 10))
		}))
		b, e := newTencentCOSBackend(Config{COS: TencentCOSConfig{SecretID: "id", SecretKey: "key", Bucket: "bucket", BucketURL: srv.URL}})
		if e != nil {
			t.Fatal(e)
		}
		if b.Provider() != ProviderCOS || b.Bucket() != "bucket" {
			t.Fatal("provider")
		}
		tag, e := b.Put(context.Background(), preparedUpload{UploadRequest: UploadRequest{Reader: strings.NewReader("pdf"), Size: 3, Metadata: map[string]string{"owner": "alice", "": "bad", "x-cos-meta-x": "y"}}, Key: "a.pdf", Content: "application/pdf"})
		if fail && e == nil {
			t.Fatal("expected HTTP error")
		}
		if !fail && (e != nil || tag != "tag") {
			t.Fatal(tag, e)
		}
		srv.Close()
	}
}

func TestProviderConfiguration(t *testing.T) {
	for i := 0; i < 4; i++ {
		c := Config{MinIO: MinIOConfig{Endpoint: "127.0.0.1:1", AccessKeyID: "id", SecretAccessKey: "key", Bucket: "bucket"}}
		switch i {
		case 0:
			c.MinIO.AccessKeyID = ""
		case 1:
			c.MinIO.SecretAccessKey = ""
		case 2:
			c.MinIO.Bucket = ""
		case 3:
			c.MinIO.Endpoint = ":bad"
		}
		if _, e := newMinIOBackend(c); e == nil {
			t.Fatal(i)
		}
	}
	for i := 0; i < 4; i++ {
		c := Config{OSS: OSSConfig{Endpoint: "http://127.0.0.1:1", AccessKeyID: "id", AccessKeySecret: "key", Bucket: "bucket"}}
		switch i {
		case 0:
			c.OSS.AccessKeyID = ""
		case 1:
			c.OSS.AccessKeySecret = ""
		case 2:
			c.OSS.Bucket = ""
		case 3:
			c.OSS.Bucket = "!"
		}
		if _, e := newOSSBackend(c); e == nil {
			t.Fatal(i)
		}
	}
	for i := 0; i < 3; i++ {
		c := Config{Qiniu: QiniuConfig{AccessKey: "id", SecretKey: "key", Bucket: "bucket", Region: "z0"}}
		switch i {
		case 0:
			c.Qiniu.SecretKey = ""
		case 1:
			c.Qiniu.Bucket = ""
		}
		b, e := newQiniuBackend(c)
		if i < 2 && e == nil {
			t.Fatal(i)
		}
		if i == 2 && (e != nil || b.Provider() != ProviderQiniu || b.Bucket() != "bucket") {
			t.Fatal(e)
		}
	}
	for i := 0; i < 5; i++ {
		c := Config{COS: TencentCOSConfig{SecretID: "id", SecretKey: "key", Bucket: "bucket", Region: "region"}}
		switch i {
		case 0:
			c.COS.SecretKey = ""
		case 1:
			c.COS.Bucket = ""
		case 2:
			c.COS.Region = ""
		case 3:
			c.COS.BucketURL = ":%"
		}
		_, e := newTencentCOSBackend(c)
		if i < 4 && e == nil {
			t.Fatal(i)
		}
		if i == 4 && e != nil {
			t.Fatal(e)
		}
	}
	m := qiniuMetadata(map[string]string{"a": "b", "x:q": "z", "x-qn-meta-k": "v", " ": "v", "nil": ""})
	if len(m) != 3 || m["x-qn-meta-a"] != "b" {
		t.Fatal(m)
	}
	if qiniuMetadata(nil) != nil {
		t.Fatal("nil")
	}
}

func TestUploadBoundaries(t *testing.T) {
	cfg := Config{MaxSizeMB: 1, AllowedExtensions: []string{"PDF", " ", ".unknownx"}, AllowedContentTypes: []string{"application/pdf", " "}}
	for _, r := range []UploadRequest{{Filename: "a.pdf", Size: 1048577}, {Filename: "name", Size: 1}, {Filename: "a.unknownx", Size: 1}} {
		if ValidateUpload(cfg, r) == nil {
			t.Fatal(r)
		}
	}
	u := &uploader{cfg: normalizeConfig(cfg), backend: fakeBackend{}}
	if _, e := u.Upload(context.Background(), UploadRequest{}); e == nil {
		t.Fatal("nil reader")
	}
	if _, e := u.Upload(context.Background(), UploadRequest{Reader: strings.NewReader("x")}); e == nil {
		t.Fatal("invalid")
	}
	r, e := u.Upload(context.Background(), UploadRequest{Reader: strings.NewReader("x"), Filename: "a.pdf", ContentType: "application/pdf", Size: 1, ObjectKey: "/fixed"})
	if e != nil || r.Key != "fixed" {
		t.Fatal(r, e)
	}
	if publicURL("https://host/", "/x") != "https://host/x" || NewObjectKey(" ", "a.pdf") == "" {
		t.Fatal("key")
	}
	for _, f := range []string{"a.doc", "a.mkv", "a.m4a"} {
		if DetectContentType(f) == "" {
			t.Fatal(f)
		}
	}
	if normalizeContentType("a/b; bad parameter") != "a/b" || normalizeContentType("bad mime") != "bad mime" {
		t.Fatal("mime")
	}
	if maxSizeBytes(Config{}) != 100*1024*1024 {
		t.Fatal("size")
	}
	if _, e := NewUploader(Config{Provider: "unknown"}); e == nil {
		t.Fatal("provider")
	}
}
