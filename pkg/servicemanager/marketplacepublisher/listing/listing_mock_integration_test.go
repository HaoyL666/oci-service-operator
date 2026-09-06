/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package listing

import (
	"path/filepath"
	"testing"

	marketplacepublishersdk "github.com/oracle/oci-go-sdk/v65/marketplacepublisher"
	marketplacepublisherv1beta1 "github.com/oracle/oci-service-operator/api/marketplacepublisher/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationListingLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "listing_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Listing OCI mock: %v", err)
		}
	})
	resource := &marketplacepublisherv1beta1.Listing{Spec: marketplacepublisherv1beta1.ListingSpec{
		CompartmentId: "ocid1.compartment.oc1..replay", Name: "osok-replay-marketplace-listing",
		ListingType: string(marketplacepublishersdk.ListingTypeOciApplication), PackageType: string(marketplacepublishersdk.PackageTypeStack),
	}}
	ocimock.InitializeResource(resource, "mock-listing")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := marketplacepublishersdk.MarketplacePublisherClient{BaseClient: session.BaseClient()}
	manager := &ListingServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newListingRuntimeHooks(manager, sdkClient)
	client := wrapListingGeneratedClient(hooks, defaultListingServiceClient{ServiceClient: generatedruntime.NewServiceClient[*marketplacepublisherv1beta1.Listing](buildListingGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
