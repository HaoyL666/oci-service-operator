/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package jmsplugin

import (
	"path/filepath"
	"testing"

	jmssdk "github.com/oracle/oci-go-sdk/v65/jms"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationJmsPluginLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "jmsplugin_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close JmsPlugin OCI mock: %v", err)
		}
	})
	resource := makeJmsPluginResource()
	ocimock.InitializeResource(resource, "mock-jmsplugin")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := jmssdk.JavaManagementServiceClient{BaseClient: session.BaseClient()}
	client := newJmsPluginServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
