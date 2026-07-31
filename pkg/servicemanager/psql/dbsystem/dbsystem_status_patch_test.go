/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package dbsystem

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	psqlsdk "github.com/oracle/oci-go-sdk/v65/psql"
	psqlv1beta1 "github.com/oracle/oci-service-operator/api/psql/v1beta1"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
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

func TestFirstStatusPatchPersistsObservedRegionalDurabilityFalse(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := psqlv1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("add PSQL API to scheme: %v", err)
	}

	testEnv := &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "..", "config", "crd", "bases", "psql.oracle.com_dbsystems.yaml")},
		},
	}
	config, err := testEnv.Start()
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}
	t.Cleanup(func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("stop envtest: %v", err)
		}
	})

	kubeClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("create envtest client: %v", err)
	}

	resource := &psqlv1beta1.DbSystem{
		ObjectMeta: metav1.ObjectMeta{Name: "status-false", Namespace: metav1.NamespaceDefault},
		Spec: psqlv1beta1.DbSystemSpec{
			DisplayName:   "status-false",
			CompartmentId: "ocid1.compartment.oc1..example",
			DbVersion:     "14",
			Shape:         "PostgreSQL.VM.Standard.E5.Flex",
			StorageDetails: psqlv1beta1.DbSystemStorageDetails{
				AvailabilityDomain:  "AD-1",
				IsRegionallyDurable: false,
				SystemType:          "OCI_OPTIMIZED_STORAGE",
			},
			Credentials: psqlv1beta1.DbSystemCredentials{Username: "postgres"},
			NetworkDetails: psqlv1beta1.DbSystemNetworkDetails{
				SubnetId: "ocid1.subnet.oc1..example",
			},
		},
	}
	if err := kubeClient.Create(ctx, resource); err != nil {
		t.Fatalf("create DbSystem: %v", err)
	}

	current := &psqlv1beta1.DbSystem{}
	if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(resource), current); err != nil {
		t.Fatalf("get initial DbSystem: %v", err)
	}
	base := current.DeepCopy()
	current.Status.OsokStatus = shared.OSOKStatus{Message: "observed"}
	current.Status.StorageDetails = psqlv1beta1.DbSystemStorageDetailsObservedState{
		AvailabilityDomain:  "AD-1",
		IsRegionallyDurable: common.Bool(false),
		SystemType:          "OCI_OPTIMIZED_STORAGE",
	}
	if err := kubeClient.Status().Patch(ctx, current, client.MergeFrom(base)); err != nil {
		t.Fatalf("patch DbSystem status: %v", err)
	}

	observed := &psqlv1beta1.DbSystem{}
	if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(resource), observed); err != nil {
		t.Fatalf("get patched DbSystem: %v", err)
	}
	if observed.Status.StorageDetails.IsRegionallyDurable == nil || *observed.Status.StorageDetails.IsRegionallyDurable {
		t.Fatalf("stored status.storageDetails.isRegionallyDurable = %#v, want non-nil false", observed.Status.StorageDetails.IsRegionallyDurable)
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
