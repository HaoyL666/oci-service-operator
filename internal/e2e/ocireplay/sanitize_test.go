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

func TestSanitizerRedactsSensitiveJSONKeysAcrossNamingStyles(t *testing.T) {
	t.Parallel()

	sanitizer := newSanitizer(nil)
	body, encoding, err := sanitizer.body([]byte(`{
		"idcsAccessToken":"idcs-value",
		"security_token":"security-value",
		"refresh-token":"refresh-value",
		"nested":{"privateKey":"private-value"},
		"ownerUserName":"person-name",
		"lastUpdatedBy":"person-id",
		"displayName":"safe-value"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if encoding != "json" {
		t.Fatalf("encoding = %q, want json", encoding)
	}
	for _, secret := range []string{"idcs-value", "security-value", "refresh-value", "private-value", "person-name", "person-id"} {
		if strings.Contains(body, secret) {
			t.Fatalf("sanitized body contains %q: %s", secret, body)
		}
	}
	if !strings.Contains(body, `"displayName":"safe-value"`) {
		t.Fatalf("sanitized body lost ordinary value: %s", body)
	}
}

func TestSanitizerPreservesOIDCConfigurationShapeAndRedactsSensitiveChildren(t *testing.T) {
	t.Parallel()

	sanitizer := newSanitizer(nil)
	body, encoding, err := sanitizer.body([]byte(`{
		"openIdConnectTokenAuthenticationConfig": {
			"isOpenIdConnectAuthEnabled": false,
			"refreshToken": "do-not-store"
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if encoding != "json" {
		t.Fatalf("encoding = %q, want json", encoding)
	}
	if !strings.Contains(body, `"openIdConnectTokenAuthenticationConfig":{"isOpenIdConnectAuthEnabled":false,"refreshToken":"<redacted>"}`) {
		t.Fatalf("sanitized body lost OIDC configuration shape: %s", body)
	}
	if strings.Contains(body, "do-not-store") {
		t.Fatalf("sanitized body retained sensitive OIDC child: %s", body)
	}
}

func TestSanitizerPreservesImagePullSecretCollectionAndRedactsCredentials(t *testing.T) {
	t.Parallel()

	sanitizer := newSanitizer(nil)
	body, encoding, err := sanitizer.body([]byte(`{
		"imagePullSecrets": [{
			"registryEndpoint": "registry.example.test",
			"password": "do-not-store",
			"secretId": "ocid1.vaultsecret.oc1..sensitive"
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if encoding != "json" {
		t.Fatalf("encoding = %q, want json", encoding)
	}
	if !strings.Contains(body, `"imagePullSecrets":[{"password":"<redacted>","registryEndpoint":"registry.example.test","secretId":"<redacted>"}]`) {
		t.Fatalf("sanitized body lost imagePullSecrets shape: %s", body)
	}
	if strings.Contains(body, "do-not-store") || strings.Contains(body, "ocid1.vaultsecret") {
		t.Fatalf("sanitized body retained image-pull credential: %s", body)
	}
}

func TestSanitizerPreservesNullSensitiveFields(t *testing.T) {
	t.Parallel()

	sanitizer := newSanitizer(nil)
	body, encoding, err := sanitizer.body([]byte(`{"password":"sensitive","secret":null,"token":null}`))
	if err != nil {
		t.Fatal(err)
	}
	if encoding != "json" {
		t.Fatalf("encoding = %q, want json", encoding)
	}
	if body != `{"password":"<redacted>","secret":null,"token":null}` {
		t.Fatalf("body = %s", body)
	}
}

func TestSanitizerPreservesSensitiveObjectShape(t *testing.T) {
	t.Parallel()

	sanitizer := newSanitizer(nil)
	body, encoding, err := sanitizer.body([]byte(`{"createdBy":{"id":"ocid1.user.oc1..sensitive","displayName":"person@example.com"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if encoding != "json" {
		t.Fatalf("encoding = %q, want json", encoding)
	}
	if body != `{"createdBy":{"displayName":"<redacted>","id":"<redacted>"}}` {
		t.Fatalf("sanitized body = %s, want redacted object shape", body)
	}
}
