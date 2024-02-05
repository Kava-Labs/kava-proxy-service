package batchmdw

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
)

// batchResponseWriter is a custom implementation of http.ResponseWriter
// it wraps multiple request responses, collecting each individual response & their headers
// all responses are written to the underlying ResponseWriter as a marshaled JSON array in FlushResponses()
type batchResponseWriter struct {
	http.ResponseWriter

	// body is the response body for the current request
	body *bytes.Buffer
	// header is the response headers for the current request
	header http.Header

	// responses collects all request responses
	responses []*bytes.Buffer
	// responseHeader is the final headers sent in the batch response
	responseHeader http.Header
	// cacheHits tracks the number of cache hits across all requests
	cacheHits int
}

var _ http.ResponseWriter = &batchResponseWriter{}

// newBatchResponseWriter creates a new batchResponseWriter prepared to make numRequests requests
func newBatchResponseWriter(w http.ResponseWriter, numRequests int) *batchResponseWriter {
	return &batchResponseWriter{
		ResponseWriter: w,
		header:         w.Header().Clone(),
		responses:      make([]*bytes.Buffer, 0, numRequests),
		cacheHits:      0,
	}
}

// Write implements the Write method of http.ResponseWriter
// it overrides the Write method to capture the response content for the current request
func (w *batchResponseWriter) Write(b []byte) (int, error) {
	// Write to the buffer
	w.body.Write(b)
	return len(b), nil
}

// Header implements the Header method of http.ResponseWriter
// it overrides the Header method to capture the response headers for the current request
func (w *batchResponseWriter) Header() http.Header {
	return w.header
}

// WriteHeader implements the WriteHeader method of http.ResponseWriter
// it overrides the WriteHeader method to prevent proxied requests from having finalized headers
func (w *batchResponseWriter) WriteHeader(status int) {
	// TODO handle error response codes
	fmt.Printf("WRITE HEADER CALLED WITH STATUS %d\n", status)
}

// updateResponseHeader resets the current `header` value for a new request
// the headers of the first non-nil response are used as a base for the whole response headers
// cache hits are tracked and a final value for the header is sent in FlushResponses()
func (w *batchResponseWriter) updateResponseHeader() {
	// initialize all headers with the value of the first response
	if w.responseHeader == nil {
		w.responseHeader = w.header.Clone()
		// clear content length, will be set by actual Write to client
		// must be cleared in order to prevent premature end of client read
		w.responseHeader.Del("Content-Length")
		// clear cache hit header, will be set by flush()
		w.responseHeader.Del(cachemdw.CacheHeaderKey)
	}

	// track cache hits
	if cachemdw.IsCacheHitHeaders(w.header) {
		w.cacheHits += 1
	}

	// clear current headers for next request
	w.header = make(http.Header)
}

// next prepares the batchResponseWriter for the next request
func (w *batchResponseWriter) next(newBody *bytes.Buffer) http.ResponseWriter {
	if w.body != nil {
		w.responses = append(w.responses, w.body)
		w.updateResponseHeader()
	}

	w.body = newBody
	return w
}

// FlushResponses marshals all request responses into a JSON array
// and write them to the underlying ResponseWriter along with the combined headers
func (w *batchResponseWriter) FlushResponses() error {
	w.next(nil)

	// write all headers
	for k, v := range w.responseHeader.Clone() {
		for _, val := range v {
			w.ResponseWriter.Header().Set(k, val)
		}
	}

	// write cache hit header based on results of all requests
	w.ResponseWriter.Header().Set(cachemdw.CacheHeaderKey, cacheHitValue(len(w.responses), w.cacheHits))

	// marshal results into a JSON array
	rawMessages := make([]json.RawMessage, 0, len(w.responses))
	for _, r := range w.responses {
		rawMessages = append(rawMessages, json.RawMessage(r.Bytes()))
	}
	res, err := json.Marshal(rawMessages)
	if err != nil {
		return err
	}

	// write to actual ResponseWriter
	// TODO: handle error response
	w.ResponseWriter.WriteHeader(http.StatusOK)
	w.ResponseWriter.Write(res)
	return nil
}

// cacheHitValue handles the combined response's CacheHeader
func cacheHitValue(totalNum, cacheHits int) string {
	// NOTE: middleware assumes non-zero batch length.
	// totalNum should never be 0. if it is, this will indicate a cache MISS.
	if cacheHits == 0 || totalNum == 0 {
		// case 1. no results from cache => MISS
		return cachemdw.CacheMissHeaderValue
	} else if cacheHits == totalNum {
		// case 2: all results from cache => HIT
		return cachemdw.CacheHitHeaderValue
	}
	//case 3: some results from cache => PARTIAL
	return cachemdw.CachePartialHeaderValue
}
