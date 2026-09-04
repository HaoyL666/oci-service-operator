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

	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
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

func TestSyntheticJSONBodySerializesTypedModel(t *testing.T) {
	t.Parallel()
	body, err := SyntheticJSONBody(struct {
		ID string `json:"id"`
	}{ID: "resource-1"})
	if err != nil {
		t.Fatal(err)
	}
	if body != `{"id":"resource-1"}` {
		t.Fatalf("SyntheticJSONBody() = %s", body)
	}
}

func TestSyntheticWorkRequestBodyUsesCommonOCIShape(t *testing.T) {
	t.Parallel()
	body, err := SyntheticWorkRequestBody("work-1", "CREATE_THING", "SUCCEEDED", "CREATED", "Thing", "thing-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"operationType":"CREATE_THING"`, `"status":"SUCCEEDED"`, `"identifier":"thing-1"`} {
		if !strings.Contains(body, field) {
			t.Fatalf("work-request body %s does not contain %s", body, field)
		}
	}
}

func TestSeedSyntheticTrackedResourceSetsIdentityAndKnownPaths(t *testing.T) {
	t.Parallel()
	type trackedStatus struct {
		ID     string `json:"id"`
		Parent string `json:"parentId"`
		Status struct {
			OCID string `json:"ocid"`
		} `json:"status"`
	}
	type trackedResource struct {
		Spec struct {
			Parent string `json:"parentId"`
		} `json:"spec"`
		Status trackedStatus `json:"status"`
	}
	resource := &trackedResource{}
	if err := SeedSyntheticTrackedResource(resource, "ocid1.thing.oc1..synthetic", map[string]any{"parentId": "ocid1.parent.oc1..synthetic"}); err != nil {
		t.Fatal(err)
	}
	if resource.Status.ID != "ocid1.thing.oc1..synthetic" || resource.Status.Status.OCID != "ocid1.thing.oc1..synthetic" {
		t.Fatalf("seeded identity = %+v", resource.Status)
	}
	if resource.Spec.Parent != "ocid1.parent.oc1..synthetic" || resource.Status.Parent != resource.Spec.Parent {
		t.Fatalf("seeded parent path = spec %q status %q", resource.Spec.Parent, resource.Status.Parent)
	}
}

func TestPreferTrackedIdentityForUnrepresentedPathsPreservesExposedParents(t *testing.T) {
	type spec struct {
		ParentID string `json:"parentId,omitempty"`
	}
	type status struct {
		ParentID string `json:"parentId,omitempty"`
	}
	type resource struct {
		Spec   spec   `json:"spec"`
		Status status `json:"status"`
	}
	value := &resource{Spec: spec{ParentID: "ocid1.parent.oc1..test"}}
	fields := []generatedruntime.RequestField{
		{FieldName: "ParentId", RequestName: "parentId", Contribution: "path"},
		{FieldName: "MissingParentId", RequestName: "missingParentId", Contribution: "path"},
		{FieldName: "ThingId", RequestName: "thingId", Contribution: "path", PreferResourceID: true},
	}
	updated := PreferTrackedIdentityForUnrepresentedPaths(value, fields)
	if updated[0].PreferResourceID {
		t.Fatal("exposed parent path unexpectedly prefers tracked identity")
	}
	if !updated[1].PreferResourceID || !updated[2].PreferResourceID {
		t.Fatalf("unrepresented path preferences = %+v", updated)
	}
	if fields[1].PreferResourceID {
		t.Fatal("input fields were mutated")
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

func TestSyntheticCRUDResponderCanStartWithExistingResource(t *testing.T) {
	t.Parallel()
	responder := NewSyntheticCRUDResponder(SyntheticCRUDOptions{
		CollectionPath:        "/resources",
		InitiallyPresent:      true,
		PresentCollectionBody: `{"items":[{"id":"resource-1"}]}`,
	})
	request := syntheticRequest(t, http.MethodGet, "https://example.test/resources")
	response, err := responder(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != `{"items":[{"id":"resource-1"}]}` {
		t.Fatalf("existing collection body = %s", response.Body)
	}
}

func TestSyntheticWorkRequestCRUDResponderTracksLifecycle(t *testing.T) {
	t.Parallel()
	responder := NewSyntheticWorkRequestCRUDResponder(SyntheticWorkRequestCRUDOptions{
		CollectionPath:        "/resources",
		CreatedBody:           `{"id":"resource-1"}`,
		CreateWorkRequestBody: `{"status":"SUCCEEDED","operationType":"CREATE"}`,
		DeleteWorkRequestBody: `{"status":"SUCCEEDED","operationType":"DELETE"}`,
	})

	list := syntheticRequest(t, http.MethodGet, "https://example.test/resources")
	response, err := responder(list)
	if err != nil || response.Body != `{"items":[]}` {
		t.Fatalf("initial list response = %#v, %v", response, err)
	}
	create := syntheticRequest(t, http.MethodPost, "https://example.test/resources")
	response, err = responder(create)
	if err != nil || response.StatusCode != http.StatusAccepted || response.Headers.Get("Opc-Work-Request-Id") == "" {
		t.Fatalf("create response = %#v, %v", response, err)
	}
	workRequest := syntheticRequest(t, http.MethodGet, "https://example.test/workRequests/create")
	response, err = responder(workRequest)
	if err != nil || !strings.Contains(response.Body, `"CREATE"`) {
		t.Fatalf("create work-request response = %#v, %v", response, err)
	}
	item := syntheticRequest(t, http.MethodGet, "https://example.test/resources/resource-1")
	response, err = responder(item)
	if err != nil || response.Body != `{"id":"resource-1"}` {
		t.Fatalf("item response = %#v, %v", response, err)
	}
	deleteRequest := syntheticRequest(t, http.MethodDelete, "https://example.test/resources/resource-1")
	response, err = responder(deleteRequest)
	if err != nil || response.StatusCode != http.StatusAccepted {
		t.Fatalf("delete response = %#v, %v", response, err)
	}
	response, err = responder(workRequest)
	if err != nil || !strings.Contains(response.Body, `"DELETE"`) {
		t.Fatalf("delete work-request response = %#v, %v", response, err)
	}
	response, err = responder(item)
	if err != nil || response.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted item response = %#v, %v", response, err)
	}
}

func TestSyntheticWorkRequestCRUDResponderSupportsSynchronousDelete(t *testing.T) {
	t.Parallel()
	responder := NewSyntheticWorkRequestCRUDResponder(SyntheticWorkRequestCRUDOptions{
		CreatedBody:  `{"id":"resource-1"}`,
		DeleteStatus: http.StatusNoContent,
	})
	deleteRequest := syntheticRequest(t, http.MethodDelete, "https://example.test/resources/resource-1")
	response, err := responder(deleteRequest)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if response.Headers.Get("Opc-Work-Request-Id") != "" {
		t.Fatalf("synchronous delete returned work request header %q", response.Headers.Get("Opc-Work-Request-Id"))
	}
}

func TestSyntheticWorkRequestCRUDResponderSupportsSynchronousCreate(t *testing.T) {
	t.Parallel()
	responder := NewSyntheticWorkRequestCRUDResponder(SyntheticWorkRequestCRUDOptions{
		CreatedBody:  `{"id":"resource-1"}`,
		CreateStatus: http.StatusCreated,
	})
	request := syntheticRequest(t, http.MethodPost, "https://example.test/resources")
	response, err := responder(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	if response.Headers.Get("Opc-Work-Request-Id") != "" {
		t.Fatalf("synchronous create returned work request header %q", response.Headers.Get("Opc-Work-Request-Id"))
	}
}

func TestSyntheticWorkRequestCRUDResponderSupportsActionDelete(t *testing.T) {
	t.Parallel()
	responder := NewSyntheticWorkRequestCRUDResponder(SyntheticWorkRequestCRUDOptions{
		CreatedBody:            `{"id":"resource-1"}`,
		CreateWorkRequestBody:  `{"status":"SUCCEEDED"}`,
		DeleteWorkRequestBody:  `{"status":"SUCCEEDED"}`,
		DeleteActionPathMarker: "/actions/cancel",
	})
	request := syntheticRequest(t, http.MethodPost, "https://example.test/resources/resource-1/actions/cancel")
	response, err := responder(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Headers.Get("Opc-Work-Request-Id") != "ocid1.workrequest.oc1..syntheticdelete" {
		t.Fatalf("action delete work request = %q", response.Headers.Get("Opc-Work-Request-Id"))
	}
}

func syntheticRequest(t *testing.T, method string, target string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(method, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func newSyntheticRequest(t *testing.T, method string, target string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(method, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	return request
}
