package batchmdw

import (
	"bytes"
	"net/http"
)

// fakeResponseWriter is a custom implementation of http.ResponseWriter that writes all content
// to a buffer.
type fakeResponseWriter struct {
	// body is the response body for the current request
	body *bytes.Buffer
	// header is the response headers for the current request
	header http.Header
	// onErrStatus is a method for handling non-OK status responses
	onErrStatus func(status int)
}

var _ http.ResponseWriter = &fakeResponseWriter{}

// newFakeResponseWriter creates a new fakeResponseWriter that wraps the provided buffer.
func newFakeResponseWriter(buf *bytes.Buffer, onErrStatus func(status int)) *fakeResponseWriter {
	return &fakeResponseWriter{
		header:      make(http.Header),
		body:        buf,
		onErrStatus: onErrStatus,
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
	if status != http.StatusOK {
		w.onErrStatus(status)
	}
}
