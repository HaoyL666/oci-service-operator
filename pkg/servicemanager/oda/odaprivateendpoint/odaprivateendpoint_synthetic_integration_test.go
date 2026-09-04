/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package odaprivateendpoint

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	odasdk "github.com/oracle/oci-go-sdk/v65/oda"
	odav1beta1 "github.com/oracle/oci-service-operator/api/oda/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestSyntheticOdaPrivateEndpointCreateReadDelete(t *testing.T) {
	resource := newOdaPrivateEndpointResource("osok-replay-oda-endpoint")
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.odaprivateendpoint.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "oda", Resource: "OdaPrivateEndpoint", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "odaprivateendpoint_synthetic_crud.yaml"), Host: "https://oda.us-ashburn-1.oci.oraclecloud.com", BasePath: "20190506", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := odasdk.ManagementClient{BaseClient: session.BaseClient()}
	client := newOdaPrivateEndpointServiceClientWithOCIClient(sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*odav1beta1.OdaPrivateEndpoint]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *odav1beta1.OdaPrivateEndpoint) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *odav1beta1.OdaPrivateEndpoint) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created OdaPrivateEndpoint status = %+v", current.Status)
		}
		return nil
	}})
}
