/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package batchtaskenvironment

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	batchsdk "github.com/oracle/oci-go-sdk/v65/batch"
	batchv1beta1 "github.com/oracle/oci-service-operator/api/batch/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticBatchTaskEnvironmentCreateReadDelete(t *testing.T) {
	resource := &batchv1beta1.BatchTaskEnvironment{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"osok-replay-batchtaskenvironment"},"spec":{"compartmentId":"ocid1.compartment.oc1..synthetic","imageUrl":"iad.ocir.io/example/osok:synthetic","displayName":"osok-replay-task-environment"}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.batchtaskenvironment.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "batch", Resource: "BatchTaskEnvironment",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "batchtaskenvironment_synthetic_crud.yaml"), Host: "https://batch.us-ashburn-1.oci.oraclecloud.com", BasePath: "20251031", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{
			CreatedBody: createdBody, EmptyCollectionBody: "{\"items\":[]}", PresentCollectionBody: "{\"items\":[" + createdBody + "]}", CreateStatus: 0, DeleteStatus: 0,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := batchsdk.BatchComputingClient{BaseClient: session.BaseClient()}
	manager := &BatchTaskEnvironmentServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newBatchTaskEnvironmentRuntimeHooks(manager, sdkClient)
	client := wrapBatchTaskEnvironmentGeneratedClient(hooks, defaultBatchTaskEnvironmentServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*batchv1beta1.BatchTaskEnvironment](buildBatchTaskEnvironmentGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*batchv1beta1.BatchTaskEnvironment]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *batchv1beta1.BatchTaskEnvironment) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *batchv1beta1.BatchTaskEnvironment) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created BatchTaskEnvironment status has no OCI identity: %+v", current.Status)
			}
			return nil
		},
	})
}
