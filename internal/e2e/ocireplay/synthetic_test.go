/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenSDKSyntheticRequiresSyntheticProvenance(t *testing.T) {
	_, err := OpenSDKSynthetic(SDKSyntheticOptions{
		Metadata: Metadata{Service: "example", Resource: "Thing", Operations: []Operation{OperationRead}, SDKVersion: "v1", Provenance: ProvenanceRecorded},
	})
	if err == nil || !strings.Contains(err.Error(), "synthetic provenance") {
		t.Fatalf("OpenSDKSynthetic() error = %v, want synthetic provenance rejection", err)
	}
}

func TestSyntheticResponseDispatcherIsOrderedAndBounded(t *testing.T) {
	dispatcher := &syntheticResponseDispatcher{responses: []SyntheticResponse{
		{StatusCode: http.StatusCreated, Body: `{"id":"thing"}`},
		{StatusCode: http.StatusNoContent},
	}}
	request, err := http.NewRequest(http.MethodPost, "https://example.invalid/things", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := dispatcher.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated || response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("first synthetic response = status %d headers %v", response.StatusCode, response.Header)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"id":"thing"}` {
		t.Fatalf("first synthetic response body = %q", body)
	}
	request.Method = http.MethodDelete
	if _, err := dispatcher.Do(request); err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.assertConsumed(); err != nil {
		t.Fatal(err)
	}
	if _, err := dispatcher.Do(request); err == nil || !strings.Contains(err.Error(), "exhausted") {
		t.Fatalf("extra dispatch error = %v, want exhausted", err)
	}
}

func TestOpenSDKSyntheticRecordsAndReplaysExactSDKRequests(t *testing.T) {
	path := filepath.Join(t.TempDir(), "synthetic.yaml")
	metadata := Metadata{Service: "example", Resource: "Thing", Operations: []Operation{OperationRead}, SDKVersion: "v1", Provenance: ProvenanceSynthetic}
	t.Setenv(syntheticRecordEnv, "true")
	session, err := OpenSDKSynthetic(SDKSyntheticOptions{
		Path: path, Host: "https://example.invalid", BasePath: "v1", Metadata: metadata,
		Responses: []SyntheticResponse{{StatusCode: http.StatusOK, Body: `{"id":"ocid1.thing.oc1..synthetic"}`}},
	})
	if err != nil {
		t.Fatal(err)
	}
	baseClient := session.BaseClient()
	request, err := http.NewRequest(http.MethodGet, baseClient.Host+"/"+baseClient.BasePath+"/things/ocid1.thing.oc1..synthetic", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := baseClient.HTTPClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(syntheticRecordEnv, "false")
	replay, err := OpenSDKSynthetic(SDKSyntheticOptions{Path: path, Host: "https://example.invalid", BasePath: "v1", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	replayBaseClient := replay.BaseClient()
	replayRequest, err := http.NewRequest(http.MethodGet, replayBaseClient.Host+"/"+replayBaseClient.BasePath+"/things/ocid1.thing.oc1..synthetic", nil)
	if err != nil {
		t.Fatal(err)
	}
	replayResponse, err := replayBaseClient.HTTPClient.Do(replayRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = replayResponse.Body.Close()
	if err := replay.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSyntheticResponseDispatcherRejectsUnconsumedResponses(t *testing.T) {
	dispatcher := &syntheticResponseDispatcher{responses: []SyntheticResponse{{StatusCode: http.StatusOK}}}
	if err := dispatcher.assertConsumed(); err == nil || !strings.Contains(err.Error(), "consumed 0 of 1") {
		t.Fatalf("assertConsumed() error = %v", err)
	}
}

func TestSyntheticObservedBodyProjectsSpecAndObservedFields(t *testing.T) {
	body, err := SyntheticObservedBody(
		struct {
			DisplayName string `json:"displayName"`
		}{DisplayName: "example"},
		"ocid1.thing.oc1..synthetic",
		"ACTIVE",
		map[string]any{"description": "observed"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"description":"observed","displayName":"example","id":"ocid1.thing.oc1..synthetic","lifecycleState":"ACTIVE"}`
	if body != want {
		t.Fatalf("SyntheticObservedBody() = %s, want %s", body, want)
	}
}

func TestSyntheticCRUDResponderTracksCreateAndDelete(t *testing.T) {
	responder := NewSyntheticCRUDResponder(SyntheticCRUDOptions{
		CollectionPath:      "/v1/things",
		CreatedBody:         `{"id":"thing","state":"ACTIVE"}`,
		EmptyCollectionBody: `{"items":[]}`,
	})

	response, err := responder(newSyntheticRequest(t, http.MethodGet, "https://example.invalid/v1/things"))
	if err != nil || response.Body != `{"items":[]}` {
		t.Fatalf("pre-create collection response = %#v, %v", response, err)
	}
	response, err = responder(newSyntheticRequest(t, http.MethodPost, "https://example.invalid/v1/things"))
	if err != nil || response.StatusCode != http.StatusCreated {
		t.Fatalf("create response = %#v, %v", response, err)
	}
	response, err = responder(newSyntheticRequest(t, http.MethodGet, "https://example.invalid/v1/things/thing"))
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("read response = %#v, %v", response, err)
	}
	response, err = responder(newSyntheticRequest(t, http.MethodDelete, "https://example.invalid/v1/things/thing"))
	if err != nil || response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete response = %#v, %v", response, err)
	}
	response, err = responder(newSyntheticRequest(t, http.MethodGet, "https://example.invalid/v1/things/thing"))
	if err != nil || response.StatusCode != http.StatusNotFound {
		t.Fatalf("post-delete response = %#v, %v", response, err)
	}
}

func newSyntheticRequest(t *testing.T, method string, target string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(method, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	return request
}
