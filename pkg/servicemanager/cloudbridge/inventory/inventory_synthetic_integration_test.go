/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package inventory

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	cloudbridgesdk "github.com/oracle/oci-go-sdk/v65/cloudbridge"
	cloudbridgev1beta1 "github.com/oracle/oci-service-operator/api/cloudbridge/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticInventoryCreateReadDelete(t *testing.T) {
	resource := &cloudbridgev1beta1.Inventory{Spec: cloudbridgev1beta1.InventorySpec{
		CompartmentId: "ocid1.compartment.oc1..replay", DisplayName: "osok-replay-inventory",
	}}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.ocbinventory.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "cloudbridge", Resource: "Inventory", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "inventory_synthetic_crud.yaml"), Host: "https://cloudbridge.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220509", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := cloudbridgesdk.InventoryClient{BaseClient: session.BaseClient()}
	manager := &InventoryServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newInventoryRuntimeHooks(manager, sdkClient)
	client := wrapInventoryGeneratedClient(hooks, defaultInventoryServiceClient{ServiceClient: generatedruntime.NewServiceClient[*cloudbridgev1beta1.Inventory](buildInventoryGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*cloudbridgev1beta1.Inventory]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *cloudbridgev1beta1.Inventory) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *cloudbridgev1beta1.Inventory) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.DisplayName != resource.Spec.DisplayName {
				return fmt.Errorf("created Inventory status = %+v", current.Status)
			}
			return nil
		},
	})
}
