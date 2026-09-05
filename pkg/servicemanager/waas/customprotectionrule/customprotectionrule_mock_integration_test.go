/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package customprotectionrule

import (
	"path/filepath"
	"testing"

	waassdk "github.com/oracle/oci-go-sdk/v65/waas"
	waasv1beta1 "github.com/oracle/oci-service-operator/api/waas/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationCustomProtectionRuleEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "customprotectionrule_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close CustomProtectionRule OCI mock: %v", err)
		}
	})
	resource := &waasv1beta1.CustomProtectionRule{}
	ocimock.InitializeResource(resource, "mock-customprotectionrule")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := waassdk.WaasClient{BaseClient: session.BaseClient()}
	hooks := newCustomProtectionRuleDefaultRuntimeHooks(sdkClient)
	applyCustomProtectionRuleRuntimeHooks(&hooks)
	manager := &CustomProtectionRuleServiceManager{Log: loggerutil.OSOKLogger{}}
	client := wrapCustomProtectionRuleGeneratedClient(hooks, defaultCustomProtectionRuleServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*waasv1beta1.CustomProtectionRule](buildCustomProtectionRuleGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
