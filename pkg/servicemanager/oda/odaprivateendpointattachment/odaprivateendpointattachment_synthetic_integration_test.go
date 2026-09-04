/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package odaprivateendpointattachment

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

func TestSyntheticOdaPrivateEndpointAttachmentCreateReadDelete(t *testing.T) {
	resource := newOdaPrivateEndpointAttachmentResource()
	resource.Status.CompartmentId = "ocid1.compartment.oc1..synthetic"
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.odaprivateendpointattachment.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "oda", Resource: "OdaPrivateEndpointAttachment", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "odaprivateendpointattachment_synthetic_crud.yaml"), Host: "https://oda.us-ashburn-1.oci.oraclecloud.com", BasePath: "20190506", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := odasdk.ManagementClient{BaseClient: session.BaseClient()}
	client := newOdaPrivateEndpointAttachmentServiceClientWithOCIClient(sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*odav1beta1.OdaPrivateEndpointAttachment]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *odav1beta1.OdaPrivateEndpointAttachment) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *odav1beta1.OdaPrivateEndpointAttachment) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created OdaPrivateEndpointAttachment status = %+v", current.Status)
		}
		return nil
	}})
}
