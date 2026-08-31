/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package budget

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	budgetsdk "github.com/oracle/oci-go-sdk/v65/budget"
	"github.com/oracle/oci-go-sdk/v65/common"
	budgetv1beta1 "github.com/oracle/oci-service-operator/api/budget/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

type replaySigner struct{}

func (replaySigner) Sign(*http.Request) error { return nil }

func TestRecordedBudgetCreateUpdateDelete(t *testing.T) {
	cassette, err := ocireplay.Open(ocireplay.Options{
		Mode: ocireplay.ModeReplay,
		Path: filepath.Join("testdata", "recordings", "budget_crud.yaml"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cassette.Close(); err != nil {
			t.Errorf("close cassette: %v", err)
		}
	})

	baseClient := common.DefaultBaseClientWithSigner(replaySigner{})
	baseClient.Host = "https://usage.us-ashburn-1.oci.oraclecloud.com"
	baseClient.BasePath = "20190111"
	noRetry := common.NoRetryPolicy()
	baseClient.Configuration.RetryPolicy = &noRetry
	sdkClient := budgetsdk.BudgetClient{BaseClient: baseClient}
	cassette.Attach(&sdkClient.BaseClient)

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
	if err := cassette.Close(); err != nil {
		t.Fatal(err)
	}
}
