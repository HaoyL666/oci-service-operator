/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package analyticsinstance

import (
	"path/filepath"
	"testing"

	analyticssdk "github.com/oracle/oci-go-sdk/v65/analytics"
	analyticsv1beta1 "github.com/oracle/oci-service-operator/api/analytics/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationAnalyticsInstanceLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "analyticsinstance_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close AnalyticsInstance OCI mock: %v", err)
		}
	})
	resource := &analyticsv1beta1.AnalyticsInstance{Spec: analyticsv1beta1.AnalyticsInstanceSpec{
		Name: "osok-replay-analytics", CompartmentId: "ocid1.compartment.oc1..replay",
		FeatureSet:  "ENTERPRISE_ANALYTICS",
		Capacity:    analyticsv1beta1.AnalyticsInstanceCapacity{CapacityType: "OLPU_COUNT", CapacityValue: 2},
		LicenseType: "LICENSE_INCLUDED", Description: "OSOK synthetic analytics instance",
	}}
	ocimock.InitializeResource(resource, "mock-analyticsinstance")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
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
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
