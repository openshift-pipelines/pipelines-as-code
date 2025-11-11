package http

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
)

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls.
func newHTTPTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

// MakeHTTPTestClient creates a test HTTP client from a config map.
func MakeHTTPTestClient(config map[string]map[string]string) *http.Client {
	return newHTTPTestClient(func(req *http.Request) *http.Response {
		resp := &http.Response{}
		for k, v := range config {
			if k == req.URL.String() {
				code, _ := strconv.Atoi(v["code"])
				resp = &http.Response{
					StatusCode: code,
					Header:     make(http.Header),
				}
				if body, ok := v["body"]; ok {
					resp.Body = io.NopCloser(bytes.NewBufferString(body))
				}
			}
		}
		return resp
	})
}

// RoundTripFunc is a function adapter to implement http.RoundTripper interface.
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip implements the http.RoundTripper interface.
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}
