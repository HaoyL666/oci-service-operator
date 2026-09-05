/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package addresslist

import (
	"path/filepath"
	"testing"

	waassdk "github.com/oracle/oci-go-sdk/v65/waas"
	waasv1beta1 "github.com/oracle/oci-service-operator/api/waas/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationAddressListEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "addresslist_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close AddressList OCI mock: %v", err)
		}
	})
	resource := &waasv1beta1.AddressList{}
	ocimock.InitializeResource(resource, "mock-addresslist")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := waassdk.WaasClient{BaseClient: session.BaseClient()}
	hooks := newAddressListDefaultRuntimeHooks(sdkClient)
	applyAddressListRuntimeHooks(&hooks, sdkClient, nil)
	manager := &AddressListServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}}
	client := wrapAddressListGeneratedClient(hooks, defaultAddressListServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*waasv1beta1.AddressList](buildAddressListGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
