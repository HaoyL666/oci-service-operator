/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package cccupgradeschedule

import (
	"path/filepath"
	"testing"

	computecloudatcustomersdk "github.com/oracle/oci-go-sdk/v65/computecloudatcustomer"
	computecloudatcustomerv1beta1 "github.com/oracle/oci-service-operator/api/computecloudatcustomer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationCccUpgradeScheduleEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "cccupgradeschedule_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close CccUpgradeSchedule OCI mock: %v", err)
		}
	})
	resource := &computecloudatcustomerv1beta1.CccUpgradeSchedule{}
	ocimock.InitializeResource(resource, "mock-cccupgradeschedule")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := computecloudatcustomersdk.ComputeCloudAtCustomerClient{BaseClient: session.BaseClient()}
	manager := &CccUpgradeScheduleServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}}
	hooks := newCccUpgradeScheduleRuntimeHooks(manager, sdkClient)
	client := wrapCccUpgradeScheduleGeneratedClient(hooks, defaultCccUpgradeScheduleServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*computecloudatcustomerv1beta1.CccUpgradeSchedule](buildCccUpgradeScheduleGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
