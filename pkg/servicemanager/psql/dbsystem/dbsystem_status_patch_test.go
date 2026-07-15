/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package dbsystem

import (
	"encoding/json"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	psqlsdk "github.com/oracle/oci-go-sdk/v65/psql"
	psqlv1beta1 "github.com/oracle/oci-service-operator/api/psql/v1beta1"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestFirstStatusPatchPreservesObservedRegionalDurabilityFalse(t *testing.T) {
	t.Parallel()

	oldObj := &psqlv1beta1.DbSystem{}
	newObj := oldObj.DeepCopy()
	newObj.Status.StorageDetails = psqlv1beta1.DbSystemStorageDetailsObservedState{
		AvailabilityDomain:  "AD-1",
		IsRegionallyDurable: common.Bool(false),
		SystemType:          "OCI_OPTIMIZED_STORAGE",
	}

	patch, err := client.MergeFrom(oldObj).Data(newObj)
	if err != nil {
		t.Fatalf("create status merge patch: %v", err)
	}

	var document map[string]any
	if err := json.Unmarshal(patch, &document); err != nil {
		t.Fatalf("decode status merge patch: %v", err)
	}
	status, ok := document["status"].(map[string]any)
	if !ok {
		t.Fatalf("merge patch status = %#v, want object", document["status"])
	}
	storageDetails, ok := status["storageDetails"].(map[string]any)
	if !ok {
		t.Fatalf("merge patch storageDetails = %#v, want object", status["storageDetails"])
	}
	if got, ok := storageDetails["isRegionallyDurable"].(bool); !ok || got {
		t.Fatalf("merge patch isRegionallyDurable = %#v, want false", storageDetails["isRegionallyDurable"])
	}
}

func TestProjectDbSystemPreservesObservedRegionalDurabilityFalse(t *testing.T) {
	t.Parallel()

	resource := testDbSystemResource()
	observed := sdkDbSystem("ocid1.dbsystem.oc1..ad-local", "sample-db", psqlsdk.DbSystemLifecycleStateCreating)
	observed.StorageDetails = psqlsdk.OciOptimizedStorageDetails{
		AvailabilityDomain:  common.String("AD-1"),
		IsRegionallyDurable: common.Bool(false),
	}

	_, err := (manualDbSystemServiceClient{log: discardDbSystemLogger()}).projectDbSystem(resource, observed, shared.Provisioning)
	if err != nil {
		t.Fatalf("project DbSystem: %v", err)
	}
	if resource.Status.StorageDetails.IsRegionallyDurable == nil || *resource.Status.StorageDetails.IsRegionallyDurable {
		t.Fatalf("status.storageDetails.isRegionallyDurable = %#v, want false", resource.Status.StorageDetails.IsRegionallyDurable)
	}
}
