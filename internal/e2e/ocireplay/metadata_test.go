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

func TestCassetteMetadataRoundTripAndExpectation(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "metadata.yaml")
	metadata := Metadata{
		Service:    "budget",
		Resource:   "Budget",
		Operations: []Operation{OperationCreate, OperationRead},
		SDKVersion: "v65.110.0",
		Provenance: ProvenanceRecorded,
	}
	recorder, err := Open(Options{
		Mode:     ModeRecord,
		Path:     path,
		Metadata: &metadata,
		Delegate: dispatcherFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, "https://example.test/resources", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recorder.Do(request); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}

	replay, err := Open(Options{Mode: ModeReplay, Path: path, Metadata: &metadata})
	if err != nil {
		t.Fatal(err)
	}
	got := replay.Metadata()
	if got == nil || !metadataEqual(metadata, *got) {
		t.Fatalf("Metadata() = %#v, want %#v", got, metadata)
	}
	got.Operations[0] = OperationDelete
	if metadataEqual(*got, *replay.Metadata()) {
		t.Fatal("Metadata() returned mutable cassette state")
	}
	if _, err := replay.Do(request); err != nil {
		t.Fatal(err)
	}
	if err := replay.Close(); err != nil {
		t.Fatal(err)
	}
	wrong := metadata
	wrong.Resource = "Alarm"
	if _, err := Open(Options{Mode: ModeReplay, Path: path, Metadata: &wrong}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("Open(wrong metadata) error = %v", err)
	}
}

func TestCassetteMetadataValidation(t *testing.T) {
	t.Parallel()

	valid := Metadata{
		Service:    "budget",
		Resource:   "Budget",
		Operations: []Operation{OperationCreate},
		SDKVersion: "v65.110.0",
		Provenance: ProvenanceRecorded,
	}
	tests := []struct {
		name     string
		metadata Metadata
		contains string
	}{
		{name: "service", metadata: func() Metadata { value := valid; value.Service = "Budget Service"; return value }(), contains: "service"},
		{name: "resource", metadata: func() Metadata { value := valid; value.Resource = ""; return value }(), contains: "resource"},
		{name: "operations", metadata: func() Metadata { value := valid; value.Operations = nil; return value }(), contains: "operations"},
		{name: "duplicate operation", metadata: func() Metadata {
			value := valid
			value.Operations = []Operation{OperationRead, OperationRead}
			return value
		}(), contains: "duplicated"},
		{name: "unknown operation", metadata: func() Metadata { value := valid; value.Operations = []Operation{"replace"}; return value }(), contains: "unsupported"},
		{name: "sdk version", metadata: func() Metadata { value := valid; value.SDKVersion = ""; return value }(), contains: "sdkVersion"},
		{name: "provenance", metadata: func() Metadata { value := valid; value.Provenance = "guessed"; return value }(), contains: "provenance"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Open(Options{
				Mode:     ModeRecord,
				Path:     filepath.Join(t.TempDir(), "recording.yaml"),
				Metadata: &tc.metadata,
				Delegate: dispatcherFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
				}),
			})
			if err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("Open() error = %v, want containing %q", err, tc.contains)
			}
		})
	}
}
