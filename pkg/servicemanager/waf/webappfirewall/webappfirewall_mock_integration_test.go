/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package webappfirewall

import (
	"path/filepath"
	"testing"

	wafsdk "github.com/oracle/oci-go-sdk/v65/waf"
	wafv1beta1 "github.com/oracle/oci-service-operator/api/waf/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationWebAppFirewallEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "webappfirewall_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close WebAppFirewall OCI mock: %v", err)
		}
	})
	resource := &wafv1beta1.WebAppFirewall{}
	ocimock.InitializeResource(resource, "mock-webappfirewall")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := wafsdk.WafClient{BaseClient: session.BaseClient()}
	client := newWebAppFirewallServiceClientWithOCIClient(loggerutil.OSOKLogger{}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
