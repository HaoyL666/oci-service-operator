/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package publication

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

func TestSyntheticPublicationCreateReadDelete(t *testing.T) {
	resource := testPublicationResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.marketplacepublication.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "marketplace", Resource: "Publication", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "publication_synthetic_crud.yaml"), Host: "https://marketplace.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181001", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, EmptyCollectionBody: `[]`, PresentCollectionBody: `[` + createdBody + `]`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := marketplacesdk.MarketplaceClient{BaseClient: session.BaseClient()}
	client := newPublicationServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*marketplacev1beta1.Publication]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *marketplacev1beta1.Publication) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *marketplacev1beta1.Publication) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created Publication status = %+v", current.Status)
		}
		return nil
	}})
}
