/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package budget

import (
	"context"
	"path/filepath"
	"testing"

	budgetsdk "github.com/oracle/oci-go-sdk/v65/budget"
	budgetv1beta1 "github.com/oracle/oci-service-operator/api/budget/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedBudgetCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{
		Service:    "budget",
		Resource:   "Budget",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	replay, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path:     filepath.Join("testdata", "recordings", "budget_crud.yaml"),
		Host:     "https://usage.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20190111",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := replay.Close(); err != nil {
			t.Errorf("close cassette: %v", err)
		}
	})

	sdkClient := budgetsdk.BudgetClient{BaseClient: replay.BaseClient()}

	hooks := newBudgetDefaultRuntimeHooks(sdkClient)
	applyBudgetRuntimeHooks(&hooks)
	config := buildBudgetGeneratedRuntimeConfig(&BudgetServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
	}, hooks)
	client := defaultBudgetServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*budgetv1beta1.Budget](config),
	}

	resource := makeSpecBudget()
	resource.Spec.CompartmentId = "ocid1.compartment.oc1..replay"
	resource.Spec.DisplayName = "osok-recorded-budget"
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful {
		t.Fatalf("create response = %#v", response)
	}
	if got, want := string(resource.Status.OsokStatus.Ocid), "ocid1.replay.oc1..cassette2"; got != want {
		t.Fatalf("created OCID = %q, want %q", got, want)
	}
	if got := resource.Status.LifecycleState; got != "ACTIVE" {
		t.Fatalf("created lifecycle state = %q", got)
	}

	resource.Spec.DisplayName = "osok-recorded-budget-updated"
	response, err = client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful {
		t.Fatalf("update response = %#v", response)
	}
	if got := resource.Status.DisplayName; got != resource.Spec.DisplayName {
		t.Fatalf("updated status displayName = %q, want %q", got, resource.Spec.DisplayName)
	}

	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("delete did not confirm OCI resource removal")
	}
	if err := replay.Close(); err != nil {
		t.Fatal(err)
	}
}
