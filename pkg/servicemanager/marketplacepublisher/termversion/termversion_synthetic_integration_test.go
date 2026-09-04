/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package termversion

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	marketplacepublishersdk "github.com/oracle/oci-go-sdk/v65/marketplacepublisher"
	marketplacepublisherv1beta1 "github.com/oracle/oci-service-operator/api/marketplacepublisher/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestSyntheticTermVersionCreateReadDelete(t *testing.T) {
	resource := newTermVersionResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.termversion.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "marketplacepublisher", Resource: "TermVersion", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "termversion_synthetic_crud.yaml"), Host: "https://marketplace-publisher.us-ashburn-1.oci.oraclecloud.com", BasePath: "20241201", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CollectionPath: "/20241201/terms/" + testTermID + "/versions", CreatedBody: createdBody, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := marketplacepublishersdk.MarketplacePublisherClient{BaseClient: session.BaseClient()}
	credentials := fakeTermVersionCredentialClient{secrets: map[string]map[string][]byte{"default/term-content": {termVersionDefaultContentSecretKey: []byte(testTermContent)}}}
	client := newTestTermVersionClient(sdkClient, credentials)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*marketplacepublisherv1beta1.TermVersion]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *marketplacepublisherv1beta1.TermVersion) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *marketplacepublisherv1beta1.TermVersion) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created TermVersion status = %+v", current.Status)
		}
		return nil
	}})
}
