/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package schedule

import (
	"path/filepath"
	"testing"

	resourceschedulersdk "github.com/oracle/oci-go-sdk/v65/resourcescheduler"
	resourceschedulerv1beta1 "github.com/oracle/oci-service-operator/api/resourcescheduler/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationScheduleEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "schedule_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Schedule OCI mock: %v", err)
		}
	})
	resource := &resourceschedulerv1beta1.Schedule{}
	ocimock.InitializeResource(resource, "mock-schedule")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := resourceschedulersdk.ScheduleClient{BaseClient: session.BaseClient()}
	hooks := newScheduleRuntimeHooksWithOCIClient(sdkClient)
	applyScheduleRuntimeHooks(&hooks)
	manager := &ScheduleServiceManager{Log: loggerutil.OSOKLogger{}}
	client := wrapScheduleGeneratedClient(hooks, defaultScheduleServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*resourceschedulerv1beta1.Schedule](buildScheduleGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
