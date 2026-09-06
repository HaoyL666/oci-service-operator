/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package databasetoolsconnection

import (
	"path/filepath"
	"testing"

	databasetoolssdk "github.com/oracle/oci-go-sdk/v65/databasetools"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationDatabaseToolsConnectionLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "databasetoolsconnection_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close DatabaseToolsConnection OCI mock: %v", err)
		}
	})
	resource := makeGenericJDBCResource()
	ocimock.InitializeResource(resource, "mock-databasetoolsconnection")
	sdkClient := databasetoolssdk.DatabaseToolsClient{BaseClient: session.BaseClient()}
	client := newDatabaseToolsConnectionServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, nil, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
