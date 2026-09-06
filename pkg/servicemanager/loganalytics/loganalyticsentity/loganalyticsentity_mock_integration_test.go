/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package loganalyticsentity

import (
	"path/filepath"
	"testing"

	loganalyticssdk "github.com/oracle/oci-go-sdk/v65/loganalytics"
	loganalyticsv1beta1 "github.com/oracle/oci-service-operator/api/loganalytics/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// Contract evidence: recorded composite-path CRUD, production service manager, and real OCI SDK serialization.
func TestMockIntegrationLogAnalyticsEntityCompositeCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "loganalyticsentity_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close LogAnalyticsEntity OCI mock: %v", err)
		}
	})
	resource := &loganalyticsv1beta1.LogAnalyticsEntity{}
	ocimock.InitializeResource(resource, "mock-loganalyticsentity")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := loganalyticssdk.LogAnalyticsClient{BaseClient: session.BaseClient()}
	hooks := newLogAnalyticsEntityRuntimeHooksWithOCIClient(sdkClient)
	applyLogAnalyticsEntityRuntimeHooks(&hooks, sdkClient, nil)
	manager := &LogAnalyticsEntityServiceManager{}
	client := wrapLogAnalyticsEntityGeneratedClient(hooks, defaultLogAnalyticsEntityServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*loganalyticsv1beta1.LogAnalyticsEntity](buildLogAnalyticsEntityGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
