/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package offer

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	offersdk "github.com/oracle/oci-go-sdk/v65/marketplaceprivateoffer"
	offerv1beta1 "github.com/oracle/oci-service-operator/api/marketplaceprivateoffer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticOfferCreateReadDelete(t *testing.T) {
	resource := newOfferRuntimeTestResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.marketplaceoffer.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "marketplaceprivateoffer", Resource: "Offer", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "offer_synthetic_crud.yaml"), Host: "https://marketplace-private-offer.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220901", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := offersdk.OfferClient{BaseClient: session.BaseClient()}
	manager := &OfferServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newOfferRuntimeHooksWithOCIClient(sdkClient)
	applyOfferRuntimeHooks(&hooks, sdkClient, nil)
	client := wrapOfferGeneratedClient(hooks, defaultOfferServiceClient{ServiceClient: generatedruntime.NewServiceClient[*offerv1beta1.Offer](buildOfferGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*offerv1beta1.Offer]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *offerv1beta1.Offer) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *offerv1beta1.Offer) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created Offer status = %+v", current.Status)
		}
		return nil
	}})
}
