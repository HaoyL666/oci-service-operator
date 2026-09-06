/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package acceptedagreement

import (
	"path/filepath"
	"testing"

	marketplacesdk "github.com/oracle/oci-go-sdk/v65/marketplace"
	marketplacev1beta1 "github.com/oracle/oci-service-operator/api/marketplace/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationAcceptedAgreementLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "acceptedagreement_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close AcceptedAgreement OCI mock: %v", err)
		}
	})
	resource := &marketplacev1beta1.AcceptedAgreement{Spec: marketplacev1beta1.AcceptedAgreementSpec{
		CompartmentId: "ocid1.compartment.oc1..replay", ListingId: "ocid1.appcataloglisting.oc1..replay",
		PackageVersion: "1.0", AgreementId: "ocid1.marketplaceagreement.oc1..replay", Signature: "synthetic-signature", DisplayName: "osok-replay-agreement",
	}}
	ocimock.InitializeResource(resource, "mock-acceptedagreement")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := marketplacesdk.MarketplaceClient{BaseClient: session.BaseClient()}
	manager := &AcceptedAgreementServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newAcceptedAgreementRuntimeHooks(manager, sdkClient)
	client := wrapAcceptedAgreementGeneratedClient(hooks, defaultAcceptedAgreementServiceClient{ServiceClient: generatedruntime.NewServiceClient[*marketplacev1beta1.AcceptedAgreement](buildAcceptedAgreementGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
