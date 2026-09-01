/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
)

func TestOpenSDKReplayConfiguresCredentialFreeBaseClient(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "sdk.yaml")
	content := `version: 1
metadata:
  service: budget
  resource: Budget
  operations: [read]
  sdkVersion: v65.110.0
  provenance: recorded
interactions:
  - request:
      method: GET
      host: usage.example.test
      path: /20190111/budgets
    response:
      statusCode: 200
      body: '[]'
      encoding: json
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	metadata := Metadata{
		Service:    "budget",
		Resource:   "Budget",
		Operations: []Operation{OperationRead},
		SDKVersion: "v65.110.0",
		Provenance: ProvenanceRecorded,
	}
	session, err := OpenSDKReplay(SDKReplayOptions{
		Path:     path,
		Host:     "https://usage.example.test/",
		BasePath: "/20190111/",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	baseClient := session.BaseClient()
	if baseClient.Host != "https://usage.example.test" || baseClient.BasePath != "20190111" {
		t.Fatalf("base client host/path = %q/%q", baseClient.Host, baseClient.BasePath)
	}
	request, err := http.NewRequest(http.MethodGet, baseClient.Host+"/"+baseClient.BasePath+"/budgets", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := baseClient.HTTPClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "[]" {
		t.Fatalf("response body = %q, want []", body)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenSDKReplayRejectsInvalidHostAndMetadata(t *testing.T) {
	t.Parallel()

	metadata := Metadata{
		Service:    "budget",
		Resource:   "Budget",
		Operations: []Operation{OperationRead},
		SDKVersion: "v65.110.0",
		Provenance: ProvenanceRecorded,
	}
	for _, host := range []string{"", "usage.example.test", "file:///tmp/replay", "https://usage.example.test/path"} {
		_, err := OpenSDKReplay(SDKReplayOptions{Path: "unused", Host: host, Metadata: metadata})
		if err == nil || !strings.Contains(err.Error(), "host") {
			t.Fatalf("OpenSDKReplay(host=%q) error = %v", host, err)
		}
	}
}

func TestOpenSDKRecordAttachesAndPublishesSanitizedCassette(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recorded.yaml")
	baseClient := common.DefaultBaseClientWithSigner(unsignedReplaySigner{})
	baseClient.HTTPClient = dispatcherFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"ocid1.example.oc1..sensitive"}`)),
			Request:    request,
		}, nil
	})
	metadata := Metadata{
		Service:    "example",
		Resource:   "Example",
		Operations: []Operation{OperationRead},
		SDKVersion: "v65.110.0",
		Provenance: ProvenanceRecorded,
	}
	session, err := OpenSDKRecord(SDKRecordOptions{
		Path:       path,
		Metadata:   metadata,
		BaseClient: &baseClient,
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodGet, "https://example.test/resources/ocid1.example.oc1..sensitive", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := baseClient.HTTPClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(response.Body); err != nil {
		t.Fatal(err)
	}
	secondaryClient := common.DefaultBaseClientWithSigner(unsignedReplaySigner{})
	if err := session.Attach(&secondaryClient); err != nil {
		t.Fatal(err)
	}
	secondaryRequest, err := http.NewRequest(http.MethodGet, "https://related.example.test/resources/ocid1.related.oc1..sensitive", nil)
	if err != nil {
		t.Fatal(err)
	}
	secondaryResponse, err := secondaryClient.HTTPClient.Do(secondaryRequest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(secondaryResponse.Body); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "ocid1.example") || !strings.Contains(string(content), "provenance: recorded") {
		t.Fatalf("recording was not sanitized or metadata was lost:\n%s", content)
	}
	if !strings.Contains(string(content), "related.example.test") {
		t.Fatalf("secondary base client interaction was not recorded:\n%s", content)
	}
}

func TestOpenSDKRecordDoesNotReplaceCassetteBeforeClose(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recorded.yaml")
	const original = "previous reviewed cassette\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	baseClient := common.DefaultBaseClientWithSigner(unsignedReplaySigner{})
	baseClient.HTTPClient = dispatcherFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Request: request}, nil
	})
	session, err := OpenSDKRecord(SDKRecordOptions{
		Path: path,
		Metadata: Metadata{
			Service:    "example",
			Resource:   "Example",
			Operations: []Operation{OperationRead},
			SDKVersion: "v65.110.0",
			Provenance: ProvenanceRecorded,
		},
		BaseClient: &baseClient,
		Overwrite:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = session
	request, err := http.NewRequest(http.MethodGet, "https://example.test/resources", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := baseClient.HTTPClient.Do(request); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != original {
		t.Fatalf("cassette changed before Close(): %q", content)
	}
}
