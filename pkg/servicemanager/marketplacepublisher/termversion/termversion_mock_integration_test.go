/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package termversion

import (
	"path/filepath"
	"testing"

	marketplacepublishersdk "github.com/oracle/oci-go-sdk/v65/marketplacepublisher"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationTermVersionLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "termversion_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close TermVersion OCI mock: %v", err)
		}
	})
	resource := newTermVersionResource()
	ocimock.InitializeResource(resource, "mock-termversion")
	sdkClient := marketplacepublishersdk.MarketplacePublisherClient{BaseClient: session.BaseClient()}
	credentials := fakeTermVersionCredentialClient{secrets: map[string]map[string][]byte{"default/term-content": {termVersionDefaultContentSecretKey: []byte(testTermContent)}}}
	client := newTestTermVersionClient(sdkClient, credentials)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
