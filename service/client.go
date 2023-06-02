package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"net/http"
)

const (
	DatabaseStatusPath = "/status/database"
)

// ProxyServiceClient provides a client
// for making requests and decoding responses
// to the proxy service API
type ProxyServiceClient struct {
	*http.Client
	config            ProxyServiceClientConfig
	DebugLogResponses bool
}

// ProxyServiceClientConfig wraps values used to
// create a new ProxyServiceClient
type ProxyServiceClientConfig struct {
	ProxyServiceHostname string
	DebugLogResponses    bool
}

// NewProxyServiceClient creates a new ProxyServiceClient
// using the provided config, returning the client and error (if any)
func NewProxyServiceClient(config ProxyServiceClientConfig) (*ProxyServiceClient, error) {
	httpClient := &http.Client{}
	return &ProxyServiceClient{
		Client:            httpClient,
		DebugLogResponses: config.DebugLogResponses,
		config:            config,
	}, nil
}

// GetDatabaseStatus calls `DatabaseStatusPath` to
// get metadata related to proxy service database operations
// such as proxied request metrics compaction and partitioning
func (c *ProxyServiceClient) GetDatabaseStatus(ctx context.Context) (DatabaseStatusResponse, error) {
	var response DatabaseStatusResponse
	url := c.config.ProxyServiceHostname + DatabaseStatusPath

	request, err := CreateRequest(http.MethodGet, url, nil)

	if err != nil {
		return response, err
	}

	err = Call(*c, request, &response)

	return response, err
}

// RequestError provides additional details about the failed request.
type RequestError struct {
	message    string
	URL        string
	StatusCode int
}

// Error implements the error interface for RequestError.
func (err *RequestError) Error() string {
	return err.message
}

// NewError creates a new RequestError
func NewError(message, url string, statusCode int) error {
	return &RequestError{message, url, statusCode}
}

// CreateRequest isolates duplicate code in creating http search request.
func CreateRequest(method string, path string, params interface{}) (*http.Request, error) {
	var buf bytes.Buffer
	var req *http.Request
	err := json.NewEncoder(&buf).Encode(&params)
	if err != nil {
		return req, err
	}
	req, err = http.NewRequest(method, path, &buf)
	if err != nil {
		return req, &RequestError{
			URL:     path,
			message: err.Error(),
		}
	}
	return req, nil
}

// Call makes an http request to a JSON HTTP api
// decoding the JSON response to the result interface if non-nil
// returning error (if any)
func Call(client ProxyServiceClient, request *http.Request, result interface{}) error {
	response, err := client.Do(request)

	if err != nil {
		return &RequestError{
			URL:     request.URL.String(),
			message: err.Error(),
		}
	}

	defer response.Body.Close()

	if !(response.StatusCode >= 200 && response.StatusCode <= 299) {
		requestURL := request.URL.String()
		return &RequestError{
			StatusCode: response.StatusCode,
			URL:        requestURL,
			message:    fmt.Sprintf("request to %s error server http error %d", requestURL, response.StatusCode),
		}
	}

	// If no result is expected, don't attempt to decode a potentially
	// empty response stream and avoid incurring EOF errors
	if result == nil {
		return nil
	}
	// Check if debug is on
	if client.DebugLogResponses {
		var bodyBytes []byte
		if response.Body != nil {
			bodyBytes, err = io.ReadAll(response.Body)
			if err != nil {
				return &RequestError{
					URL:     request.URL.String(),
					message: err.Error(),
				}
			}
			fmt.Printf("Request Path %s \n Response Body %s \n  Response Status Code %d \n ", request.URL, string(bodyBytes), response.StatusCode)

		}
		// Repopulate body with the data read
		response.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		return &RequestError{
			URL:     request.URL.String(),
			message: err.Error(),
		}
	}
	return nil
}
