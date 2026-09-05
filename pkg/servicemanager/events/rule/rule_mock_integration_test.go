/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package rule

import (
	"path/filepath"
	"testing"

	eventssdk "github.com/oracle/oci-go-sdk/v65/events"
	eventsv1beta1 "github.com/oracle/oci-service-operator/api/events/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationRuleEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "rule_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Rule OCI mock: %v", err)
		}
	})
	resource := &eventsv1beta1.Rule{}
	ocimock.InitializeResource(resource, "mock-rule")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := eventssdk.EventsClient{BaseClient: session.BaseClient()}
	client := newRecordedRuleClient(sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
