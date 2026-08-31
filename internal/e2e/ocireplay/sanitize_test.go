/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"net/http"
	"strings"
	"testing"
)

func TestSanitizerAssignsPlaceholdersInCanonicalKeyOrder(t *testing.T) {
	t.Parallel()

	sanitizer := newSanitizer(nil)
	body, encoding, err := sanitizer.body([]byte(`{"zId":"ocid1.z.oc1..value","aId":"ocid1.a.oc1..value"}`))
	if err != nil {
		t.Fatal(err)
	}
	if encoding != "json" || body != `{"aId":"<ocid:1>","zId":"<ocid:2>"}` {
		t.Fatalf("sanitized body = %q (%s)", body, encoding)
	}

	sanitizer = newSanitizer(nil)
	request, err := http.NewRequest(http.MethodGet, "https://example.test/resources?z=ocid1.z.oc1..value&a=ocid1.a.oc1..value", nil)
	if err != nil {
		t.Fatal(err)
	}
	recorded, err := sanitizer.request(request, nil)
	if err != nil {
		t.Fatal(err)
	}
	if recorded.Query != "a=%3Cocid%3A1%3E&z=%3Cocid%3A2%3E" {
		t.Fatalf("sanitized query = %q", recorded.Query)
	}
}

func TestValidateBindingsRejectsAmbiguousDefinitions(t *testing.T) {
	t.Parallel()

	tests := []map[string]string{
		{"Invalid Name": "value"},
		{"name": ""},
		{"first": "same", "second": "same"},
	}
	for _, bindings := range tests {
		if err := validateBindings(bindings); err == nil {
			t.Fatalf("validateBindings(%v) error = nil", bindings)
		}
	}
	if err := validateSafeRecording([]byte("body: ocid1.compartment.oc1..raw")); err == nil || !strings.Contains(err.Error(), "ocid1.") {
		t.Fatalf("validateSafeRecording(raw OCID) error = %v", err)
	}
}
