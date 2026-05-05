package ingest

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReadOTLPBodyIdentity(t *testing.T) {
	req := &http.Request{
		Header: make(http.Header),
		Body:   io.NopCloser(strings.NewReader("plain-body")),
	}

	body, err := readOTLPBody(req)
	if err != nil {
		t.Fatalf("readOTLPBody returned error: %v", err)
	}

	if got := string(body); got != "plain-body" {
		t.Fatalf("unexpected body: got %q", got)
	}
}

func TestReadOTLPBodyGzip(t *testing.T) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write([]byte("compressed-body")); err != nil {
		t.Fatalf("write gzip payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	req := &http.Request{
		Header: make(http.Header),
		Body:   io.NopCloser(bytes.NewReader(buf.Bytes())),
	}
	req.Header.Set("Content-Encoding", "gzip")

	body, err := readOTLPBody(req)
	if err != nil {
		t.Fatalf("readOTLPBody returned error: %v", err)
	}

	if got := string(body); got != "compressed-body" {
		t.Fatalf("unexpected body: got %q", got)
	}
}

func TestReadOTLPBodyUnsupportedEncoding(t *testing.T) {
	req := &http.Request{
		Header: make(http.Header),
		Body:   io.NopCloser(strings.NewReader("body")),
	}
	req.Header.Set("Content-Encoding", "br")

	if _, err := readOTLPBody(req); err == nil {
		t.Fatal("expected error for unsupported content encoding")
	}
}
