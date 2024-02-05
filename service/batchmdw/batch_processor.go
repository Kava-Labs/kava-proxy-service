package batchmdw

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
)

type BatchProcessor struct {
	handler   http.HandlerFunc
	requests  []*http.Request
	responses []*bytes.Buffer
	header    http.Header
	cacheHits int
	mu        sync.Mutex
}

func NewBatchProcessor(handler http.HandlerFunc, reqs []*http.Request) *BatchProcessor {
	return &BatchProcessor{
		handler:   handler,
		requests:  reqs,
		responses: make([]*bytes.Buffer, len(reqs)),
		header:    nil,
		mu:        sync.Mutex{},
	}
}

func (bp *BatchProcessor) RequestAndServe(w http.ResponseWriter) error {
	wg := sync.WaitGroup{}
	for i, r := range bp.requests {
		wg.Add(1)

		go func(idx int, req *http.Request) {
			fmt.Printf("HANDLING REQUEST %d\n", idx)

			buf := new(bytes.Buffer)
			frw := newfakeResponseWriter(buf)
			bp.handler.ServeHTTP(frw, req)

			fmt.Printf("RESPONSE %d: %+v", idx, buf.String())
			bp.setResponse(idx, buf)
			bp.applyHeaders(frw.header)

			wg.Done()
		}(i, r)
	}

	fmt.Println("WAITING")
	wg.Wait()

	fmt.Println("DONE WAITING")
	// write all headers
	for k, v := range bp.header {
		for _, val := range v {
			w.Header().Set(k, val)
		}
	}

	// write cache hit header based on results of all requests
	w.Header().Set(cachemdw.CacheHeaderKey, cacheHitValue(len(bp.requests), bp.cacheHits))

	// marshal results into a JSON array
	rawMessages := make([]json.RawMessage, 0, len(bp.requests))
	for _, r := range bp.responses {
		rawMessages = append(rawMessages, json.RawMessage(r.Bytes()))
	}
	res, err := json.Marshal(rawMessages)
	if err != nil {
		return err
	}

	// TODO: handle error response
	w.WriteHeader(http.StatusOK)
	w.Write(res)

	return nil
}

func (bp *BatchProcessor) setResponse(idx int, res *bytes.Buffer) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.responses[idx] = res
}

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
	if cachemdw.IsCacheHitHeaders(bp.header) {
		bp.cacheHits += 1
	}
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
