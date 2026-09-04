/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package opsiconfiguration

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	opsisdk "github.com/oracle/oci-go-sdk/v65/opsi"
	opsiv1beta1 "github.com/oracle/oci-service-operator/api/opsi/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestSyntheticOpsiConfigurationCreateReadDelete(t *testing.T) {
	resource := makeOpsiConfigurationResource()
	resourceID := "ocid1.opsiconfiguration.oc1..synthetic"
	createdBody, err := ocireplay.SyntheticJSONBody(makeSDKOpsiConfiguration(resourceID, resource, opsisdk.OpsiConfigurationLifecycleStateActive))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(makeOpsiConfigurationWorkRequest("ocid1.workrequest.oc1..syntheticcreate", opsisdk.OperationTypeCreateOpsiConfiguration, opsisdk.OperationStatusSucceeded, opsisdk.ActionTypeCreated, resourceID))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(makeOpsiConfigurationWorkRequest("ocid1.workrequest.oc1..syntheticdelete", opsisdk.OperationTypeDeleteOpsiConfiguration, opsisdk.OperationStatusSucceeded, opsisdk.ActionTypeDeleted, resourceID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "opsi", Resource: "OpsiConfiguration", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "opsiconfiguration_synthetic_crud.yaml"), Host: "https://operationsinsights.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200630", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20200630/opsiConfigurations", CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := opsisdk.OperationsInsightsClient{BaseClient: session.BaseClient()}
	client := newTestOpsiConfigurationClient(sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*opsiv1beta1.OpsiConfiguration]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *opsiv1beta1.OpsiConfiguration) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *opsiv1beta1.OpsiConfiguration) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created OpsiConfiguration status = %+v", current.Status)
		}
		return nil
	}})
}
