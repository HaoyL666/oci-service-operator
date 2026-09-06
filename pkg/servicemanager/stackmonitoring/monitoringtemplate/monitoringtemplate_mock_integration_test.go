/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package monitoringtemplate

import (
	"path/filepath"
	"testing"

	stackmonitoringsdk "github.com/oracle/oci-go-sdk/v65/stackmonitoring"
	stackmonitoringv1beta1 "github.com/oracle/oci-service-operator/api/stackmonitoring/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationMonitoringTemplateLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "monitoringtemplate_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close MonitoringTemplate OCI mock: %v", err)
		}
	})
	resource := testMonitoringTemplate()
	ocimock.InitializeResource(resource, "mock-monitoringtemplate")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := stackmonitoringsdk.StackMonitoringClient{BaseClient: session.BaseClient()}
	manager := &MonitoringTemplateServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newMonitoringTemplateRuntimeHooks(manager, sdkClient)
	client := wrapMonitoringTemplateGeneratedClient(hooks, defaultMonitoringTemplateServiceClient{ServiceClient: generatedruntime.NewServiceClient[*stackmonitoringv1beta1.MonitoringTemplate](buildMonitoringTemplateGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
