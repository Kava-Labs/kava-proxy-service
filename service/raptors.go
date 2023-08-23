package service

// RequestInterceptors (a.k.a. 🦖🦖) are functions run by the proxy service
// to modify the original request sent by the caller or the response
// returned by the backend
type RequestInterceptor func([]byte) ([]byte, error)

