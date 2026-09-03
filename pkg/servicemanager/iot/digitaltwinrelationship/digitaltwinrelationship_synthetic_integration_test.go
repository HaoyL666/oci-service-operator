/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package digitaltwinrelationship

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	iotsdk "github.com/oracle/oci-go-sdk/v65/iot"
	iotv1beta1 "github.com/oracle/oci-service-operator/api/iot/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticDigitalTwinRelationshipCreateReadDelete(t *testing.T) {
	resource := makeDigitalTwinRelationshipResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.digitaltwinrelationship.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "iot", Resource: "DigitalTwinRelationship", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "digitaltwinrelationship_synthetic_crud.yaml"), Host: "https://iot.us-ashburn-1.oci.oraclecloud.com", BasePath: "20250531", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := iotsdk.IotClient{BaseClient: session.BaseClient()}
	client := newDigitalTwinRelationshipServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*iotv1beta1.DigitalTwinRelationship]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *iotv1beta1.DigitalTwinRelationship) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *iotv1beta1.DigitalTwinRelationship) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created DigitalTwinRelationship status = %+v", current.Status)
		}
		return nil
	}})
}
