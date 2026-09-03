/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package endpoint

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	generativeaisdk "github.com/oracle/oci-go-sdk/v65/generativeai"
	generativeaiv1beta1 "github.com/oracle/oci-service-operator/api/generativeai/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticEndpointCreateReadDelete(t *testing.T) {
	resource := &generativeaiv1beta1.Endpoint{Spec: generativeaiv1beta1.EndpointSpec{
		CompartmentId: "ocid1.compartment.oc1..replay", ModelId: "ocid1.generativeaimodel.oc1..replay",
		DedicatedAiClusterId: "ocid1.generativeaidedicatedaicluster.oc1..replay", DisplayName: "osok-replay-generative-ai-endpoint",
	}}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.generativeaiendpoint.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "generativeai", Resource: "Endpoint", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "endpoint_synthetic_crud.yaml"), Host: "https://generativeai.us-ashburn-1.oci.oraclecloud.com", BasePath: "20231130", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := generativeaisdk.GenerativeAiClient{BaseClient: session.BaseClient()}
	manager := &EndpointServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newEndpointRuntimeHooks(manager, sdkClient)
	client := wrapEndpointGeneratedClient(hooks, defaultEndpointServiceClient{ServiceClient: generatedruntime.NewServiceClient[*generativeaiv1beta1.Endpoint](buildEndpointGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*generativeaiv1beta1.Endpoint]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *generativeaiv1beta1.Endpoint) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *generativeaiv1beta1.Endpoint) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.DisplayName != resource.Spec.DisplayName {
				return fmt.Errorf("created Endpoint status = %+v", current.Status)
			}
			return nil
		},
	})
}
