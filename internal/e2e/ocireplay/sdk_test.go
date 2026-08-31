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
