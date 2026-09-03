/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package listingrevisionnote

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	marketplacepublishersdk "github.com/oracle/oci-go-sdk/v65/marketplacepublisher"
	marketplacepublisherv1beta1 "github.com/oracle/oci-service-operator/api/marketplacepublisher/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticListingRevisionNoteCreateReadDelete(t *testing.T) {
	resource := newListingRevisionNoteResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.listingrevisionnote.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "marketplacepublisher", Resource: "ListingRevisionNote",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "listingrevisionnote_synthetic_crud.yaml"), Host: "https://marketplace-publisher.us-ashburn-1.oci.oraclecloud.com", BasePath: "20241201", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := marketplacepublishersdk.MarketplacePublisherClient{BaseClient: session.BaseClient()}
	manager := &ListingRevisionNoteServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newListingRevisionNoteDefaultRuntimeHooks(sdkClient)
	applyListingRevisionNoteRuntimeHooks(&hooks)
	client := wrapListingRevisionNoteGeneratedClient(hooks, defaultListingRevisionNoteServiceClient{ServiceClient: generatedruntime.NewServiceClient[*marketplacepublisherv1beta1.ListingRevisionNote](buildListingRevisionNoteGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*marketplacepublisherv1beta1.ListingRevisionNote]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *marketplacepublisherv1beta1.ListingRevisionNote) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *marketplacepublisherv1beta1.ListingRevisionNote) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created ListingRevisionNote status = %+v", current.Status)
			}
			return nil
		},
	})
}
