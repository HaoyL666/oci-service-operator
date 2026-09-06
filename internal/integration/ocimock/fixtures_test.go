/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocimock

import (
	"net/http"
	"testing"
)

type explicitFixture struct {
	Name  string            `json:"name"`
	Count int               `json:"count"`
	Tags  map[string]string `json:"tags,omitempty"`
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
