/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package publication

import (
	"path/filepath"
	"testing"

	marketplacesdk "github.com/oracle/oci-go-sdk/v65/marketplace"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationPublicationLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "publication_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Publication OCI mock: %v", err)
		}
	})
	resource := testPublicationResource()
	ocimock.InitializeResource(resource, "mock-publication")
	sdkClient := marketplacesdk.MarketplaceClient{BaseClient: session.BaseClient()}
	client := newPublicationServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
