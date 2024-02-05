package batchmdw

import (
	"bytes"
	"fmt"
	"net/http"
)

// fakeResponseWriter is a custom implementation of http.ResponseWriter that writes all content
// to a buffer.
type fakeResponseWriter struct {
	// body is the response body for the current request
	body *bytes.Buffer
	// header is the response headers for the current request
	header http.Header
}

var _ http.ResponseWriter = &fakeResponseWriter{}

// newFakeResponseWriter creates a new fakeResponseWriter that wraps the provided buffer.
func newFakeResponseWriter(buf *bytes.Buffer) *fakeResponseWriter {
	return &fakeResponseWriter{
		header: make(http.Header),
		body:   buf,
	}
}

// Write implements the Write method of http.ResponseWriter
// it overrides the Write method to capture the response content for the current request
func (w *fakeResponseWriter) Write(b []byte) (int, error) {
	// Write to the buffer
	w.body.Write(b)
	return len(b), nil
}

// Header implements the Header method of http.ResponseWriter
// it overrides the Header method to capture the response headers for the current request
func (w *fakeResponseWriter) Header() http.Header {
	return w.header
}

// WriteHeader implements the WriteHeader method of http.ResponseWriter
// it overrides the WriteHeader method to prevent proxied requests from having finalized headers
func (w *fakeResponseWriter) WriteHeader(status int) {
	// TODO handle error response codes
	fmt.Printf("WRITE HEADER CALLED WITH STATUS %d\n", status)
}
