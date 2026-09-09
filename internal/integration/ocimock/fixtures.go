/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocimock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// TestingT is the narrow test interface needed by MustJSONFixture.
type TestingT interface {
	Helper()
	Fatalf(string, ...any)
}

// InitializeResource gives a typed CR the stable Kubernetes identity used by
// generated retry-token and checkpoint logic.
func InitializeResource(resource metav1.Object, name string) {
	if resource == nil {
		return
	}
	resource.SetName(name)
	resource.SetNamespace("default")
	resource.SetUID(types.UID(name + "-uid"))
}

// StateSequence preserves an explicitly authored sequence while allowing Go
// to infer the resource-specific OCI SDK response type at the call site.
func StateSequence[T any](states ...T) []T {
	return append([]T(nil), states...)
}

// ValidateRetryToken verifies that generatedruntime used the resource's stable
// Kubernetes identity for an OCI create request.
func ValidateRetryToken(request Request, resource metav1.Object) error {
	if resource == nil {
		return fmt.Errorf("%s %s retry-token resource is nil", request.Method, request.URL.Path)
	}
	want := string(resource.GetUID())
	if want == "" {
		return fmt.Errorf("%s %s retry-token resource UID is empty", request.Method, request.URL.Path)
	}
	return ValidateRetryTokenValue(request, want)
}

// ValidateRetryTokenValue verifies a package-owned deterministic retry token
// when the resource intentionally scopes its token beyond the raw Kubernetes
// UID.
func ValidateRetryTokenValue(request Request, want string) error {
	if strings.TrimSpace(want) == "" {
		return fmt.Errorf("%s %s expected retry token is empty", request.Method, request.URL.Path)
	}
	if got := request.Header.Get("opc-retry-token"); got != want {
		return fmt.Errorf("%s %s opc-retry-token = %q, want %q", request.Method, request.URL.Path, got, want)
	}
	return nil
}

// MustJSONFixture constructs an explicitly selected typed CR, request details,
// or OCI response model from package-owned JSON. The type argument keeps SDK
// polymorphic decoding and field names checked at the test's call site.
func MustJSONFixture[T any](test TestingT, content string) T {
	test.Helper()
	var value T
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		test.Fatalf("decode explicit %T fixture: %v", value, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			test.Fatalf("decode explicit %T fixture: trailing JSON value", value)
		}
		test.Fatalf("decode explicit %T fixture trailer: %v", value, err)
	}
	return value
}

// MustOCIResponseFixture constructs a typed SDK response body while allowing
// additive service fields that are not represented by the pinned SDK model.
func MustOCIResponseFixture[T any](test TestingT, content string) T {
	test.Helper()
	var value T
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		test.Fatalf("decode explicit OCI %T response fixture: %v", value, err)
	}
	return value
}

// MustMergeJSONFixture applies an explicitly declared JSON patch to an
// existing typed value while retaining fields omitted by the patch.
func MustMergeJSONFixture[T any](test TestingT, target *T, content string) {
	test.Helper()
	if target == nil {
		test.Fatalf("merge explicit fixture into nil target")
	}
	baseline, err := json.Marshal(*target)
	if err != nil {
		test.Fatalf("clone explicit %T fixture: %v", *target, err)
	}
	var cloned T
	if err := json.Unmarshal(baseline, &cloned); err != nil {
		test.Fatalf("clone explicit %T fixture: %v", *target, err)
	}
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cloned); err != nil {
		test.Fatalf("merge explicit %T fixture: %v", *target, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			test.Fatalf("merge explicit %T fixture: trailing JSON value", *target)
		}
		test.Fatalf("merge explicit %T fixture trailer: %v", *target, err)
	}
	*target = cloned
}

// ValidateJSONRequest decodes an SDK-serialized request body into the caller's
// explicit typed details model and compares the complete value.
func ValidateJSONRequest[T any](request Request, expected T) error {
	var actual T
	if err := DecodeJSONRequest(request, &actual); err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("%s %s details = %+v, want %+v", request.Method, request.URL.Path, actual, expected)
	}
	return nil
}

// CompareJSONValues compares typed values by their semantic JSON form. Tests
// opt into this only for SDK models containing formatting-sensitive raw JSON.
func CompareJSONValues[T any](actual, expected T) error {
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		return fmt.Errorf("marshal actual %T JSON value: %w", actual, err)
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return fmt.Errorf("marshal expected %T JSON value: %w", expected, err)
	}
	var actualValue, expectedValue any
	if err := json.Unmarshal(actualJSON, &actualValue); err != nil {
		return fmt.Errorf("decode actual %T JSON value: %w", actual, err)
	}
	if err := json.Unmarshal(expectedJSON, &expectedValue); err != nil {
		return fmt.Errorf("decode expected %T JSON value: %w", expected, err)
	}
	if !reflect.DeepEqual(actualValue, expectedValue) {
		return fmt.Errorf("typed JSON value = %s, want %s", actualJSON, expectedJSON)
	}
	return nil
}

// CompareJSONSubset compares every field explicitly declared by expected while
// allowing the SDK request model to carry additional zero-valued fields. This
// keeps package tests strict about their authored contract without forcing
// fixtures to restate unrelated value-shaped CR fields.
func CompareJSONSubset[T any](actual, expected T) error {
	actualValue, actualJSON, err := comparableJSONValue(actual)
	if err != nil {
		return fmt.Errorf("marshal actual %T JSON subset: %w", actual, err)
	}
	expectedValue, expectedJSON, err := comparableJSONValue(expected)
	if err != nil {
		return fmt.Errorf("marshal expected %T JSON subset: %w", expected, err)
	}
	if path, ok := jsonSubsetMismatch(actualValue, expectedValue, "$"); !ok {
		return fmt.Errorf("typed JSON subset mismatch at %s: actual=%s expected=%s", path, actualJSON, expectedJSON)
	}
	return nil
}

func comparableJSONValue(value any) (any, []byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, nil, err
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, nil, err
	}
	return decoded, encoded, nil
}

func jsonSubsetMismatch(actual, expected any, path string) (string, bool) {
	switch wanted := expected.(type) {
	case map[string]any:
		got, ok := actual.(map[string]any)
		if !ok {
			return path, false
		}
		for key, wantedValue := range wanted {
			// Typed SDK fixtures cannot distinguish an omitted field from an
			// explicitly authored null after unmarshalling. Nil therefore means
			// the package test did not declare an assertion for that field.
			if wantedValue == nil || key == "JsonData" || emptyJSONCollection(wantedValue) {
				continue
			}
			gotValue, found := got[key]
			if !found {
				return path + "." + key, false
			}
			if mismatch, matches := jsonSubsetMismatch(gotValue, wantedValue, path+"."+key); !matches {
				return mismatch, false
			}
		}
		return "", true
	case []any:
		got, ok := actual.([]any)
		if !ok || len(got) != len(wanted) {
			return path, false
		}
		for index := range wanted {
			if mismatch, matches := jsonSubsetMismatch(got[index], wanted[index], fmt.Sprintf("%s[%d]", path, index)); !matches {
				return mismatch, false
			}
		}
		return "", true
	default:
		return path, reflect.DeepEqual(actual, expected)
	}
}

func emptyJSONCollection(value any) bool {
	switch typed := value.(type) {
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

// ValidateDiscriminatedJSONRequest verifies an explicit polymorphic
// discriminator, removes it, and compares the remaining body with the selected
// concrete SDK details type.
func ValidateDiscriminatedJSONRequest[T any](request Request, field string, want any, expected T) error {
	var body map[string]any
	if err := json.Unmarshal(request.Body, &body); err != nil {
		return fmt.Errorf("decode discriminated %s %s request: %w", request.Method, request.URL.Path, err)
	}
	if !reflect.DeepEqual(body[field], want) {
		return fmt.Errorf("%s %s discriminator %s = %#v, want %#v", request.Method, request.URL.Path, field, body[field], want)
	}
	delete(body, field)
	normalized, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("normalize discriminated %s %s request: %w", request.Method, request.URL.Path, err)
	}
	request.Body = normalized
	return ValidateJSONRequest(request, expected)
}

// ValidateDiscriminatedJSONRequestSubset verifies a polymorphic discriminator
// and every concrete field explicitly declared by expected.
func ValidateDiscriminatedJSONRequestSubset[T any](request Request, field string, want any, expected T) error {
	var body map[string]any
	if err := json.Unmarshal(request.Body, &body); err != nil {
		return fmt.Errorf("decode discriminated %s %s request: %w", request.Method, request.URL.Path, err)
	}
	if !reflect.DeepEqual(body[field], want) {
		return fmt.Errorf("%s %s discriminator %s = %#v, want %#v", request.Method, request.URL.Path, field, body[field], want)
	}
	delete(body, field)
	normalized, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("normalize discriminated %s %s request: %w", request.Method, request.URL.Path, err)
	}
	var actual T
	if err := json.Unmarshal(normalized, &actual); err != nil {
		return fmt.Errorf("decode discriminated concrete %s %s request: %w", request.Method, request.URL.Path, err)
	}
	return CompareJSONSubset(actual, expected)
}
