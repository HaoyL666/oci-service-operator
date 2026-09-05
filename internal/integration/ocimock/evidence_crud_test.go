/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocimock

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestEvidenceCRUDResponderIsStatefulAndReusable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "widget.yaml")
	content := `version: 1
interactions:
  - request: {method: GET, path: /widgets, query: "name=demo"}
    response: {statusCode: 200, body: '{"items":[]}'}
  - request: {method: POST, path: /widgets, body: '{"displayName":"demo"}'}
    response: {statusCode: 200, body: '{"id":"widget-1","displayName":"demo","lifecycleState":"ACTIVE"}'}
  - request: {method: GET, path: /widgets/widget-1}
    response: {statusCode: 200, body: '{"id":"widget-1","displayName":"demo","lifecycleState":"ACTIVE"}'}
  - request: {method: PUT, path: /widgets/widget-1, body: '{"displayName":"updated"}'}
    response: {statusCode: 200, body: '{"id":"widget-1","displayName":"updated","lifecycleState":"ACTIVE"}'}
  - request: {method: GET, path: /widgets/widget-1}
    response: {statusCode: 200, body: '{"id":"widget-1","displayName":"updated","lifecycleState":"ACTIVE"}'}
  - request: {method: DELETE, path: /widgets/widget-1}
    response: {statusCode: 204}
  - request: {method: GET, path: /widgets/widget-1}
    response: {statusCode: 404, body: '{"code":"NotAuthorizedOrNotFound"}'}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	responder, err := NewEvidenceCRUDResponder(path)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, rawURL, body string) Response {
		t.Helper()
		parsed, err := url.Parse(rawURL)
		if err != nil {
			t.Fatal(err)
		}
		response, err := responder.Respond(Request{Method: method, URL: parsed, Body: []byte(body)})
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	request(http.MethodGet, "https://mock.invalid/widgets?name=demo", "")
	request(http.MethodPost, "https://mock.invalid/widgets", `{"displayName":"demo"}`)
	request(http.MethodGet, "https://mock.invalid/widgets/widget-1", "")
	request(http.MethodGet, "https://mock.invalid/widgets/widget-1", "")
	request(http.MethodPut, "https://mock.invalid/widgets/widget-1", `{"displayName":"updated"}`)
	request(http.MethodGet, "https://mock.invalid/widgets/widget-1", "")
	request(http.MethodDelete, "https://mock.invalid/widgets/widget-1", "")
	if response := request(http.MethodGet, "https://mock.invalid/widgets/widget-1", ""); response.StatusCode != http.StatusNotFound {
		t.Fatalf("post-delete status = %d", response.StatusCode)
	}
	if err := responder.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestEvidenceCRUDResponderRejectsRequestDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "widget.yaml")
	content := `interactions:
  - request: {method: POST, path: /widgets, body: '{"displayName":"demo"}'}
    response: {statusCode: 200, body: '{"id":"widget-1"}'}
  - request: {method: GET, path: /widgets/widget-1}
    response: {statusCode: 200, body: '{"id":"widget-1"}'}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	responder, err := NewEvidenceCRUDResponder(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse("https://mock.invalid/widgets")
	_, err = responder.Respond(Request{Method: http.MethodPost, URL: parsed, Body: []byte(`{"displayName":"drift"}`)})
	if err == nil {
		t.Fatal("expected create body drift to fail")
	}
}
