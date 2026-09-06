/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package log

import (
	"path/filepath"
	"testing"

	loggingsdk "github.com/oracle/oci-go-sdk/v65/logging"
	loggingv1beta1 "github.com/oracle/oci-service-operator/api/logging/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: recorded composite-path CRUD, production service manager, and real OCI SDK serialization.
func TestMockIntegrationLogCompositeCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "log_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Log OCI mock: %v", err)
		}
	})
	resource := &loggingv1beta1.Log{}
	ocimock.InitializeResource(resource, "mock-log")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	resource.Spec.LogGroupId = "<ocid:1>"
	sdkClient := loggingsdk.LoggingManagementClient{BaseClient: session.BaseClient()}
	client := newRecordedLogClient(sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
