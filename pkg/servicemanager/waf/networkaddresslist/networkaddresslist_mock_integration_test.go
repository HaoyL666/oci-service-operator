/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package networkaddresslist

import (
	"path/filepath"
	"testing"

	wafsdk "github.com/oracle/oci-go-sdk/v65/waf"
	wafv1beta1 "github.com/oracle/oci-service-operator/api/waf/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationNetworkAddressListEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "networkaddresslist_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close NetworkAddressList OCI mock: %v", err)
		}
	})
	resource := &wafv1beta1.NetworkAddressList{}
	ocimock.InitializeResource(resource, "mock-networkaddresslist")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := wafsdk.WafClient{BaseClient: session.BaseClient()}
	hooks := newNetworkAddressListDefaultRuntimeHooks(sdkClient)
	applyNetworkAddressListRuntimeHooks(&hooks)
	manager := &NetworkAddressListServiceManager{Log: loggerutil.OSOKLogger{}}
	client := wrapNetworkAddressListGeneratedClient(hooks, defaultNetworkAddressListServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*wafv1beta1.NetworkAddressList](buildNetworkAddressListGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
