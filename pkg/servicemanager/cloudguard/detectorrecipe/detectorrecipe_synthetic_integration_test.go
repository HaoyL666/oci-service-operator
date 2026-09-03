/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package detectorrecipe

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	cloudguardsdk "github.com/oracle/oci-go-sdk/v65/cloudguard"
	cloudguardv1beta1 "github.com/oracle/oci-service-operator/api/cloudguard/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticDetectorRecipeCreateReadDelete(t *testing.T) {
	resource := &cloudguardv1beta1.DetectorRecipe{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"osok-replay-detectorrecipe"},"spec":{"displayName":"osok-replay-detector-recipe","compartmentId":"ocid1.compartment.oc1..synthetic","description":"OSOK synthetic detector recipe"}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.detectorrecipe.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "cloudguard", Resource: "DetectorRecipe",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "detectorrecipe_synthetic_crud.yaml"), Host: "https://cloudguard-cp-api.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200131", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{
			CreatedBody: createdBody, EmptyCollectionBody: "{\"items\":[]}", PresentCollectionBody: "{\"items\":[" + createdBody + "]}", CreateStatus: 0, DeleteStatus: 0,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := cloudguardsdk.CloudGuardClient{BaseClient: session.BaseClient()}
	manager := &DetectorRecipeServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newDetectorRecipeRuntimeHooks(manager, sdkClient)
	client := wrapDetectorRecipeGeneratedClient(hooks, defaultDetectorRecipeServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*cloudguardv1beta1.DetectorRecipe](buildDetectorRecipeGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*cloudguardv1beta1.DetectorRecipe]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *cloudguardv1beta1.DetectorRecipe) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *cloudguardv1beta1.DetectorRecipe) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created DetectorRecipe status has no OCI identity: %+v", current.Status)
			}
			return nil
		},
	})
}
