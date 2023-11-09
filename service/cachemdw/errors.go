package cachemdw

import "errors"

var (
	ErrRequestIsNotCacheable  = errors.New("request is not cacheable")
	ErrResponseIsNotCacheable = errors.New("response is not cacheable")
)
