/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package apiplatforminstance

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	apiplatformsdk "github.com/oracle/oci-go-sdk/v65/apiplatform"
	apiplatformv1beta1 "github.com/oracle/oci-service-operator/api/apiplatform/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticApiPlatformInstanceCreateReadDelete(t *testing.T) {
	resource := &apiplatformv1beta1.ApiPlatformInstance{Spec: apiplatformv1beta1.ApiPlatformInstanceSpec{
		Name: "osok-replay-api-platform", CompartmentId: "ocid1.compartment.oc1..replay", Description: "synthetic API Platform instance",
	}}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.apiplatforminstance.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "apiplatform", Resource: "ApiPlatformInstance", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "apiplatforminstance_synthetic_crud.yaml"), Host: "https://apiplatform.us-ashburn-1.oci.oraclecloud.com", BasePath: "20240829", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := apiplatformsdk.ApiPlatformClient{BaseClient: session.BaseClient()}
	manager := &ApiPlatformInstanceServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newApiPlatformInstanceRuntimeHooks(manager, sdkClient)
	client := wrapApiPlatformInstanceGeneratedClient(hooks, defaultApiPlatformInstanceServiceClient{ServiceClient: generatedruntime.NewServiceClient[*apiplatformv1beta1.ApiPlatformInstance](buildApiPlatformInstanceGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*apiplatformv1beta1.ApiPlatformInstance]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *apiplatformv1beta1.ApiPlatformInstance) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *apiplatformv1beta1.ApiPlatformInstance) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.Name != resource.Spec.Name {
				return fmt.Errorf("created ApiPlatformInstance status = %+v", current.Status)
			}
			return nil
		},
	})
}
