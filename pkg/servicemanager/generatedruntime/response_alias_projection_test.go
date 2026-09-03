/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package generatedruntime

import (
	"testing"

	shared "github.com/oracle/oci-service-operator/pkg/shared"
)

type aliasedProjectionResource struct {
	Status aliasedProjectionStatus
}

type aliasedProjectionStatus struct {
	OsokStatus shared.OSOKStatus `json:"status"`
	SDKStatus  string            `json:"sdkStatus,omitempty"`
	ID         string            `json:"id,omitempty"`
}

type aliasedProjectionResponse struct {
	Body aliasedProjectionBody `presentIn:"body"`
}

type aliasedProjectionBody struct {
	Status string `json:"status"`
	ID     string `json:"id"`
}

func TestProjectResponseBodyWithAliasesPreservesOSOKStatus(t *testing.T) {
	resource := &aliasedProjectionResource{Status: aliasedProjectionStatus{
		OsokStatus: shared.OSOKStatus{Message: "preserved"},
	}}
	err := ProjectResponseBodyWithAliases(resource, aliasedProjectionResponse{
		Body: aliasedProjectionBody{Status: "ENABLED", ID: "ocid1.thing.oc1..test"},
	}, map[string]string{"status": "sdkStatus"})
	if err != nil {
		t.Fatal(err)
	}
	if resource.Status.SDKStatus != "ENABLED" || resource.Status.ID != "ocid1.thing.oc1..test" {
		t.Fatalf("aliased projection = %+v", resource.Status)
	}
	if resource.Status.OsokStatus.Message != "preserved" {
		t.Fatalf("OSOK status was overwritten: %+v", resource.Status.OsokStatus)
	}
}
