/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package offer

import (
	"path/filepath"
	"testing"

	offersdk "github.com/oracle/oci-go-sdk/v65/marketplaceprivateoffer"
	offerv1beta1 "github.com/oracle/oci-service-operator/api/marketplaceprivateoffer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationOfferLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "offer_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Offer OCI mock: %v", err)
		}
	})
	resource := newOfferRuntimeTestResource()
	ocimock.InitializeResource(resource, "mock-offer")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := offersdk.OfferClient{BaseClient: session.BaseClient()}
	manager := &OfferServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newOfferRuntimeHooksWithOCIClient(sdkClient)
	applyOfferRuntimeHooks(&hooks, sdkClient, nil)
	client := wrapOfferGeneratedClient(hooks, defaultOfferServiceClient{ServiceClient: generatedruntime.NewServiceClient[*offerv1beta1.Offer](buildOfferGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
