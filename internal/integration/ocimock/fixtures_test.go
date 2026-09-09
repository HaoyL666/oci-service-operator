/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocimock

import (
	"net/http"
	"net/url"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type explicitFixture struct {
	Name  string            `json:"name"`
	Count int               `json:"count"`
	Tags  map[string]string `json:"tags,omitempty"`
}

func TestStateSequencePreservesOrder(t *testing.T) {
	t.Parallel()

	states := StateSequence("CREATING", "ACTIVE")
	if len(states) != 2 || states[0] != "CREATING" || states[1] != "ACTIVE" {
		t.Fatalf("StateSequence() = %v", states)
	}
}

func TestMustJSONFixtureReturnsExplicitType(t *testing.T) {
	t.Parallel()
	fixture := MustJSONFixture[explicitFixture](t, "{\"name\":\"typed\",\"count\":2}")
	if fixture.Name != "typed" || fixture.Count != 2 || fixture.Tags != nil {
		t.Fatalf("fixture = %+v", fixture)
	}
}

func TestValidateJSONRequestComparesCompleteTypedValue(t *testing.T) {
	t.Parallel()
	request := Request{
		Method: http.MethodPost,
		URL:    mustTestURL(t, "https://mock.invalid/things"),
		Body:   []byte("{\"name\":\"typed\",\"count\":2}"),
	}
	if err := ValidateJSONRequest(request, explicitFixture{Name: "typed", Count: 2}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSONRequest(request, explicitFixture{Name: "typed", Count: 3}); err == nil {
		t.Fatal("ValidateJSONRequest() error = nil, want mismatch")
	}
}

func TestCompareJSONSubsetChecksDeclaredFieldsAndAllowsAdditionalFields(t *testing.T) {
	t.Parallel()
	type requestFixture struct {
		Name    *string `json:"name,omitempty"`
		Enabled *bool   `json:"enabled,omitempty"`
	}
	name := "typed"
	enabled := false
	actual := requestFixture{Name: &name, Enabled: &enabled}
	if err := CompareJSONSubset(actual, requestFixture{Name: &name}); err != nil {
		t.Fatal(err)
	}
	other := "other"
	if err := CompareJSONSubset(actual, requestFixture{Name: &other}); err == nil {
		t.Fatal("CompareJSONSubset() error = nil, want declared-field mismatch")
	}
}

func TestValidateRetryTokenMatchesResourceUID(t *testing.T) {
	resource := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{UID: types.UID("stable-resource-uid")}}
	request := Request{Method: http.MethodPost, URL: &url.URL{Path: "/resources"}, Header: http.Header{"Opc-Retry-Token": []string{"stable-resource-uid"}}}
	if err := ValidateRetryToken(request, resource); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRetryTokenRejectsMissingOrDifferentUID(t *testing.T) {
	request := Request{Method: http.MethodPost, URL: &url.URL{Path: "/resources"}, Header: make(http.Header)}
	if err := ValidateRetryToken(request, &metav1.PartialObjectMetadata{}); err == nil {
		t.Fatal("expected empty resource UID to fail")
	}
	resource := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{UID: types.UID("stable-resource-uid")}}
	if err := ValidateRetryToken(request, resource); err == nil {
		t.Fatal("expected missing retry token to fail")
	}
	request.Header.Set("opc-retry-token", "different-uid")
	if err := ValidateRetryToken(request, resource); err == nil {
		t.Fatal("expected different retry token to fail")
	}
}

func TestValidateRetryTokenValueSupportsPackageOwnedDeterministicTokens(t *testing.T) {
	t.Parallel()

	request := Request{Method: http.MethodPost, URL: &url.URL{Path: "/resources"}, Header: http.Header{"Opc-Retry-Token": []string{"scoped-token"}}}
	if err := ValidateRetryTokenValue(request, "scoped-token"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRetryTokenValue(request, ""); err == nil {
		t.Fatal("expected empty deterministic token to fail")
	}
	if err := ValidateRetryTokenValue(request, "other-token"); err == nil {
		t.Fatal("expected different deterministic token to fail")
	}
}

func TestMustMergeJSONFixturePreservesOmittedTypedFields(t *testing.T) {
	t.Parallel()
	baseline := explicitFixture{Name: "created", Count: 1, Tags: map[string]string{"phase": "create"}}
	fixture := baseline
	MustMergeJSONFixture(t, &fixture, "{\"name\":\"updated\",\"tags\":{\"phase\":\"update\"}}")
	if fixture.Name != "updated" || fixture.Count != 1 || fixture.Tags["phase"] != "update" {
		t.Fatalf("fixture = %+v", fixture)
	}
	if baseline.Tags["phase"] != "create" {
		t.Fatalf("baseline was mutated: %+v", baseline)
	}
}

func TestMustOCIResponseFixtureAllowsAdditiveServiceFields(t *testing.T) {
	t.Parallel()
	fixture := MustOCIResponseFixture[explicitFixture](t, "{\"name\":\"typed\",\"count\":2,\"futureField\":true}")
	if fixture.Name != "typed" || fixture.Count != 2 || fixture.Tags != nil {
		t.Fatalf("fixture = %+v", fixture)
	}
}

func TestValidateDiscriminatedJSONRequestUsesConcreteDetails(t *testing.T) {
	t.Parallel()
	request := Request{
		Method: http.MethodPost,
		URL:    mustTestURL(t, "https://mock.invalid/things"),
		Body:   []byte("{\"type\":\"NAMED\",\"name\":\"typed\",\"count\":2}"),
	}
	if err := ValidateDiscriminatedJSONRequest(request, "type", "NAMED", explicitFixture{Name: "typed", Count: 2}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateDiscriminatedJSONRequestSubsetAllowsAdditionalConcreteFields(t *testing.T) {
	t.Parallel()
	request := Request{
		Method: http.MethodPost,
		URL:    mustTestURL(t, "https://mock.invalid/things"),
		Body:   []byte("{\"type\":\"NAMED\",\"name\":\"typed\",\"count\":2}"),
	}
	if err := ValidateDiscriminatedJSONRequestSubset(request, "type", "NAMED", struct {
		Name string `json:"name"`
	}{Name: "typed"}); err != nil {
		t.Fatal(err)
	}
}
