/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package managementstation

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	osmanagementhubsdk "github.com/oracle/oci-go-sdk/v65/osmanagementhub"
	osmanagementhubv1beta1 "github.com/oracle/oci-service-operator/api/osmanagementhub/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticManagementStationCreateReadDelete(t *testing.T) {
	resource := testManagementStationResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.managementstation.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "osmanagementhub", Resource: "ManagementStation",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "managementstation_synthetic_crud.yaml"), Host: "https://osmh.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220901", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := osmanagementhubsdk.ManagementStationClient{BaseClient: session.BaseClient()}
	manager := &ManagementStationServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newManagementStationDefaultRuntimeHooks(sdkClient)
	applyManagementStationRuntimeHooks(manager, &hooks)
	client := wrapManagementStationGeneratedClient(hooks, defaultManagementStationServiceClient{ServiceClient: generatedruntime.NewServiceClient[*osmanagementhubv1beta1.ManagementStation](buildManagementStationGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*osmanagementhubv1beta1.ManagementStation]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *osmanagementhubv1beta1.ManagementStation) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *osmanagementhubv1beta1.ManagementStation) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created ManagementStation status = %+v", current.Status)
			}
			return nil
		},
	})
}
