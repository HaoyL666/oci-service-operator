/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package listingrevisionnote

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
func TestMockIntegrationListingRevisionNoteLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "listingrevisionnote_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close ListingRevisionNote OCI mock: %v", err)
		}
	})
	resource := newListingRevisionNoteResource()
	ocimock.InitializeResource(resource, "mock-listingrevisionnote")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := marketplacepublishersdk.MarketplacePublisherClient{BaseClient: session.BaseClient()}
	manager := &ListingRevisionNoteServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newListingRevisionNoteDefaultRuntimeHooks(sdkClient)
	applyListingRevisionNoteRuntimeHooks(&hooks)
	client := wrapListingRevisionNoteGeneratedClient(hooks, defaultListingRevisionNoteServiceClient{ServiceClient: generatedruntime.NewServiceClient[*marketplacepublisherv1beta1.ListingRevisionNote](buildListingRevisionNoteGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
