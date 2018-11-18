package currly_test

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/DrDoofenshmirtz/currly"
)

func TestBuildAndCallCurlWithSimpleURL(t *testing.T) {
	var req *http.Request

	c := connectorFunc(func(r *http.Request) (*http.Response, error) {
		req = r
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Request:    r,
			Body:       ioutil.NopCloser(strings.NewReader("{\"success\": true}")),
		}

		return resp, nil
	})
	curl, err := currly.Builder(c).GET().HTTPS().Localhost().Port(17500).Build()

	if err != nil {
		t.Fatalf("Building the cURL function returned an unexpected error: %v", err)
	}

	sc, res, err := curl()

	if err != nil {
		t.Fatalf("Calling the cURL function returned an unexpected error: %v", err)
	}

	if req == nil {
		t.Fatalf("Calling the cURL function should send a request.")
	}

	if http.StatusOK != sc {
		t.Errorf("Unexpected HTTP status code (expected: %v, actual: %v).", http.StatusOK, sc)
	}

	if "{\n  \"success\": true\n}" != res {
		t.Errorf("Unexpected result (expected: %v, actual: %v).", "", res)
	}

	if "https" != req.URL.Scheme {
		t.Errorf("Unexpected scheme (expected: %v, actual: %v).", "https", req.URL.Scheme)
	}

	if "localhost" != req.URL.Hostname() {
		t.Errorf("Unexpected host name (expected: %v, actual: %v).", "localhost", req.URL.Hostname())
	}

	if "17500" != req.URL.Port() {
		t.Errorf("Unexpected port (expected: %v, actual: %v).", 17500, req.URL.Port())
	}

	if http.MethodGet != req.Method {
		t.Errorf("Unexpected request method (expected: %v, actual: %v).", http.MethodGet, req.Method)
	}
}

type connectorFunc func(r *http.Request) (*http.Response, error)

func (f connectorFunc) Send(r *http.Request) (*http.Response, error) {
	return f(r)
}
