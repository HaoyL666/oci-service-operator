/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package analyticsinstance

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	analyticssdk "github.com/oracle/oci-go-sdk/v65/analytics"
	analyticsv1beta1 "github.com/oracle/oci-service-operator/api/analytics/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Analytics instances allocate paid analytics capacity and have long service
// lifecycles, so this scenario exercises the SDK/runtime contract synthetically.
func TestSyntheticAnalyticsInstanceCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{
		Service: "analytics", Resource: "AnalyticsInstance",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path: filepath.Join("testdata", "recordings", "analyticsinstance_synthetic_crud.yaml"),
		Host: "https://analytics.us-ashburn-1.oci.oraclecloud.com", BasePath: "20190331", Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := analyticssdk.AnalyticsClient{BaseClient: session.BaseClient()}
	manager := &AnalyticsInstanceServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newAnalyticsInstanceRuntimeHooks(manager, sdkClient)
	client := wrapAnalyticsInstanceGeneratedClient(hooks, defaultAnalyticsInstanceServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*analyticsv1beta1.AnalyticsInstance](
			buildAnalyticsInstanceGeneratedRuntimeConfig(manager, hooks),
		),
	})
	resource := &analyticsv1beta1.AnalyticsInstance{Spec: analyticsv1beta1.AnalyticsInstanceSpec{
		Name: "osok-replay-analytics", CompartmentId: "ocid1.compartment.oc1..replay",
		FeatureSet:  "ENTERPRISE_ANALYTICS",
		Capacity:    analyticsv1beta1.AnalyticsInstanceCapacity{CapacityType: "OLPU_COUNT", CapacityValue: 2},
		LicenseType: "LICENSE_INCLUDED", Description: "OSOK synthetic analytics instance",
	}}
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*analyticsv1beta1.AnalyticsInstance]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close,
		Timeout:       time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *analyticsv1beta1.AnalyticsInstance) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *analyticsv1beta1.AnalyticsInstance) error {
			if current.Status.Name != "osok-replay-analytics" || current.Status.LifecycleState != "ACTIVE" {
				return fmt.Errorf("created AnalyticsInstance status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *analyticsv1beta1.AnalyticsInstance) {
			current.Spec.Description = "OSOK synthetic analytics instance updated"
		},
		ValidateUpdated: func(current *analyticsv1beta1.AnalyticsInstance) error {
			if current.Status.Description != "OSOK synthetic analytics instance updated" {
				return fmt.Errorf("updated AnalyticsInstance status = %+v", current.Status)
			}
			return nil
		},
	})
}
