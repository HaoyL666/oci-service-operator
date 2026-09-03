/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package acceptedagreement

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	marketplacesdk "github.com/oracle/oci-go-sdk/v65/marketplace"
	marketplacev1beta1 "github.com/oracle/oci-service-operator/api/marketplace/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticAcceptedAgreementCreateReadDelete(t *testing.T) {
	resource := &marketplacev1beta1.AcceptedAgreement{Spec: marketplacev1beta1.AcceptedAgreementSpec{
		CompartmentId: "ocid1.compartment.oc1..replay", ListingId: "ocid1.appcataloglisting.oc1..replay",
		PackageVersion: "1.0", AgreementId: "ocid1.marketplaceagreement.oc1..replay", Signature: "synthetic-signature", DisplayName: "osok-replay-agreement",
	}}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.acceptedagreement.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "marketplace", Resource: "AcceptedAgreement", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "acceptedagreement_synthetic_crud.yaml"), Host: "https://marketplace.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181001", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, EmptyCollectionBody: `[]`, PresentCollectionBody: `[` + createdBody + `]`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := marketplacesdk.MarketplaceClient{BaseClient: session.BaseClient()}
	manager := &AcceptedAgreementServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newAcceptedAgreementRuntimeHooks(manager, sdkClient)
	client := wrapAcceptedAgreementGeneratedClient(hooks, defaultAcceptedAgreementServiceClient{ServiceClient: generatedruntime.NewServiceClient[*marketplacev1beta1.AcceptedAgreement](buildAcceptedAgreementGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*marketplacev1beta1.AcceptedAgreement]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *marketplacev1beta1.AcceptedAgreement) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *marketplacev1beta1.AcceptedAgreement) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.ListingId != resource.Spec.ListingId {
				return fmt.Errorf("created AcceptedAgreement status = %+v", current.Status)
			}
			return nil
		},
	})
}
