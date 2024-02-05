package batchmdw

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
)

// fakeResponseWriter is a custom implementation of http.ResponseWriter
type fakeResponseWriter struct {
	http.ResponseWriter

	body   *bytes.Buffer
	header http.Header

	responses      []*bytes.Buffer
	cacheHits      int
	responseHeader http.Header
}

func newFakeResponseWriter(w http.ResponseWriter, len int) *fakeResponseWriter {
	return &fakeResponseWriter{
		ResponseWriter: w,
		header:         make(http.Header),
		responses:      make([]*bytes.Buffer, 0, len),
		cacheHits:      0,
	}
}

// Write implements the Write method of http.ResponseWriter
func (w *fakeResponseWriter) Write(b []byte) (int, error) {
	// Write to the buffer
	w.body.Write(b)
	return len(b), nil
}

func (w *fakeResponseWriter) Header() http.Header {
	return w.header
}

func (w *fakeResponseWriter) updateResponseHeader() {
	fmt.Printf("incoming headers: %+v\n", w.header)
	fmt.Printf("my headers before:%+v\n", w.responseHeader)

	// initialize all headers with the value of the first response
	if w.responseHeader == nil {
		w.responseHeader = w.header.Clone()
		// clear content length, will be set by actual Write to client
		w.responseHeader.Del("Content-Length")
		// clear cache hit header, will be set by flush()
		w.responseHeader.Del(cachemdw.CacheHeaderKey)
	}

	// track cache hits
	if cachemdw.IsCacheHitHeaders(w.header) {
		w.cacheHits += 1
	}

	fmt.Printf("my headers after:%+v\n", w.responseHeader)

	// clear current headers for next request
	w.header = make(http.Header)
}

func (w *fakeResponseWriter) next(newBody *bytes.Buffer) http.ResponseWriter {
	if w.body != nil {
		w.responses = append(w.responses, w.body)
	}
	// w.updateResponseHeader()
	if cachemdw.IsCacheHitHeaders(w.Header()) {
		w.cacheHits += 1
	}
	w.Header().Del(cachemdw.CacheHeaderKey)
	w.Header().Del("Content-Length")

	w.body = newBody
	return w
}

func (w *fakeResponseWriter) FlushResponses() error {
	w.next(nil)

	// write all headers
	// headers := w.Header()
	// for k, v := range w.responseHeader {
	// 	for _, val := range v {
	// 		w.ResponseWriter.Header().Set(k, val)
	// 	}
	// }

	// write cache hit header based on results of all requests
	w.ResponseWriter.Header().Set(cachemdw.CacheHeaderKey, cacheHitValue(len(w.responses), w.cacheHits))

	w.ResponseWriter.Header().Set("PIRTLE-HEADER-KEY", "hellloooooo?!")

	fmt.Println("yoo?!")
	fmt.Printf("desired headers: %+v\n", w.responseHeader)
	fmt.Printf("written headers:%+v\n", w.ResponseWriter.Header())

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
	w.ResponseWriter.Write(res)
	return nil
}

func cacheHitValue(totalNum, cacheHits int) string {
	// TODO: what is the result if totalNum == 0?
	if cacheHits == 0 {
		// case 1. no results from cache => MISS
		return cachemdw.CacheMissHeaderValue
	} else if cacheHits == totalNum {
		// case 2: all results from cache => HIT
		return cachemdw.CacheHitHeaderValue
	}
	//case 3: some results from cache => PARTIAL
	return cachemdw.CachePartialHeaderValue
}
