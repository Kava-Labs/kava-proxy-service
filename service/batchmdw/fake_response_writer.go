package batchmdw

import (
	"bytes"
	"net/http"
)

type ErrorHandler = func(status int, headers http.Header, body *bytes.Buffer)

// fakeResponseWriter is a custom implementation of http.ResponseWriter that writes all content
// to a buffer.
type fakeResponseWriter struct {
	// body is the response body for the current request
	body *bytes.Buffer
	// header is the response headers for the current request
	header http.Header
	// onErrorHandler is a method for handling non-OK status responses
	onErrorHandler ErrorHandler
}

var _ http.ResponseWriter = &fakeResponseWriter{}

// newFakeResponseWriter creates a new fakeResponseWriter that wraps the provided buffer.
func newFakeResponseWriter(buf *bytes.Buffer, onErrorHandler ErrorHandler) *fakeResponseWriter {
	return &fakeResponseWriter{
		header:         make(http.Header),
		body:           buf,
		onErrorHandler: onErrorHandler,
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
		w.onErrorHandler(status, w.header, w.body)
	}
}
