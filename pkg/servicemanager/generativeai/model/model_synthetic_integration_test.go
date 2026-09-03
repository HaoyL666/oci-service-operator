/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package model

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

func TestSyntheticModelCreateReadDelete(t *testing.T) {
	resource := makeSpecModel()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.generativeaimodel.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "generativeai", Resource: "Model",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "model_synthetic_crud.yaml"), Host: "https://generativeai.us-ashburn-1.oci.oraclecloud.com", BasePath: "20231130", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := generativeaisdk.GenerativeAiClient{BaseClient: session.BaseClient()}
	manager := &ModelServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newModelDefaultRuntimeHooks(sdkClient)
	applyModelRuntimeHooks(&hooks)
	client := wrapModelGeneratedClient(hooks, defaultModelServiceClient{ServiceClient: generatedruntime.NewServiceClient[*generativeaiv1beta1.Model](buildModelGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*generativeaiv1beta1.Model]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *generativeaiv1beta1.Model) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *generativeaiv1beta1.Model) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created Model status = %+v", current.Status)
			}
			return nil
		},
	})
}
