package http

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
)

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func newHTTPTestClient(fn roundTripFunc) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(fn),
	}
}

func MakeHTTPTestClient(t *testing.T, statusCode int, body string) *http.Client {
	httpTestClient := newHTTPTestClient(func(req *http.Request) *http.Response {
		// Test request parameters
		return &http.Response{
			StatusCode: statusCode,
			// Send response to be tested
			Body: ioutil.NopCloser(bytes.NewBufferString(body)),
			// Must be set to non-nil value or it panics
			Header: make(http.Header),
		}
	})
	return httpTestClient
}

// roundTripFunc .
type roundTripFunc func(req *http.Request) *http.Response

// RoundTrip .
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}
