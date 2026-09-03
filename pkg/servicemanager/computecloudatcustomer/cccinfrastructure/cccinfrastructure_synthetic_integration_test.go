/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package cccinfrastructure

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	computecloudatcustomersdk "github.com/oracle/oci-go-sdk/v65/computecloudatcustomer"
	computecloudatcustomerv1beta1 "github.com/oracle/oci-service-operator/api/computecloudatcustomer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticCccInfrastructureCreateReadDelete(t *testing.T) {
	resource := &computecloudatcustomerv1beta1.CccInfrastructure{Spec: computecloudatcustomerv1beta1.CccInfrastructureSpec{
		DisplayName: "osok-replay-ccc-infrastructure", CompartmentId: "ocid1.compartment.oc1..replay",
		SubnetId: "ocid1.subnet.oc1..replay", Description: "synthetic Compute Cloud at Customer infrastructure",
	}}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.cccinfrastructure.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "computecloudatcustomer", Resource: "CccInfrastructure", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "cccinfrastructure_synthetic_crud.yaml"), Host: "https://ccc.us-ashburn-1.oci.oraclecloud.com", BasePath: "20221208", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := computecloudatcustomersdk.ComputeCloudAtCustomerClient{BaseClient: session.BaseClient()}
	manager := &CccInfrastructureServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newCccInfrastructureRuntimeHooks(manager, sdkClient)
	client := wrapCccInfrastructureGeneratedClient(hooks, defaultCccInfrastructureServiceClient{ServiceClient: generatedruntime.NewServiceClient[*computecloudatcustomerv1beta1.CccInfrastructure](buildCccInfrastructureGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*computecloudatcustomerv1beta1.CccInfrastructure]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *computecloudatcustomerv1beta1.CccInfrastructure) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *computecloudatcustomerv1beta1.CccInfrastructure) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.DisplayName != resource.Spec.DisplayName {
				return fmt.Errorf("created CccInfrastructure status = %+v", current.Status)
			}
			return nil
		},
	})
}
