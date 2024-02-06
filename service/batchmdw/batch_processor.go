package batchmdw

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
)

// BatchProcessor makes multiple requests to the underlying handler and then combines all the
// responses into a single response.
// It assumes all individual responses are valid json. Each response is then marshaled into an array.
type BatchProcessor struct {
	handler   http.HandlerFunc
	requests  []*http.Request
	responses []*bytes.Buffer
	header    http.Header
	cacheHits int
	status    int
	mu        sync.Mutex
}

// NewBatchProcessor creates a BatchProcessor for combining the responses of reqs to the handler
func NewBatchProcessor(handler http.HandlerFunc, reqs []*http.Request) *BatchProcessor {
	return &BatchProcessor{
		handler:   handler,
		requests:  reqs,
		responses: make([]*bytes.Buffer, len(reqs)),
		header:    nil,
		status:    http.StatusOK,
		mu:        sync.Mutex{},
	}
}

// RequestAndServe concurrently sends each request to the underlying handler
// Responses are then collated into a JSON array and written to the ResponseWriter
func (bp *BatchProcessor) RequestAndServe(w http.ResponseWriter) error {
	wg := sync.WaitGroup{}
	for i, r := range bp.requests {
		wg.Add(1)

		go func(idx int, req *http.Request) {

			buf := new(bytes.Buffer)
			frw := newFakeResponseWriter(buf, bp.setErrStatus)
			bp.handler.ServeHTTP(frw, req)

			bp.setResponse(idx, buf)
			bp.applyHeaders(frw.header)

			wg.Done()
		}(i, r)
	}

	wg.Wait()

	// write all headers
	for k, v := range bp.header {
		for _, val := range v {
			w.Header().Set(k, val)
		}
	}

	// write cache hit header based on results of all requests
	w.Header().Set(cachemdw.CacheHeaderKey, cacheHitValue(len(bp.requests), bp.cacheHits))

	// return error status if any sub-request returned a non-200 response
	if bp.status != http.StatusOK {
		w.WriteHeader(bp.status)
		w.Write(nil)
		return nil
	}

	// marshal results into a JSON array
	rawMessages := make([]json.RawMessage, 0, len(bp.requests))
	for _, r := range bp.responses {
		rawMessages = append(rawMessages, json.RawMessage(r.Bytes()))
	}
	res, err := json.Marshal(rawMessages)
	if err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)

	return nil
}

// setResponse is a thread-safe method to set the response for the query with index idx
func (bp *BatchProcessor) setResponse(idx int, res *bytes.Buffer) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.responses[idx] = res
}

// applyHeaders is a thread-safe method for combining new response headers with existing results.
// the headers of the first response are used, except for Content-Length and the cache hit status.
// Cache hits are tracked so a representative value can be set after all responses are received.
func (bp *BatchProcessor) applyHeaders(h http.Header) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// initialize all headers with the value of the first response
	if bp.header == nil {
		bp.header = h.Clone()
		// clear content length, will be set by actual Write to client
		// must be cleared in order to prevent premature end of client read
		bp.header.Del("Content-Length")
		// clear cache hit header, will be set by flush()
		bp.header.Del(cachemdw.CacheHeaderKey)
	}

	// track cache hits
	if cachemdw.IsCacheHitHeaders(h) {
		bp.cacheHits += 1
	}
}

// SetErrStatus tracks an error status code if any request returns a non-200 response
func (bp *BatchProcessor) setErrStatus(status int) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.status = status
}

// cacheHitValue handles determining the value for the combined response's CacheHeader
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
