/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package discoveryjob

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	datasafesdk "github.com/oracle/oci-go-sdk/v65/datasafe"
	datasafev1beta1 "github.com/oracle/oci-service-operator/api/datasafe/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticDiscoveryJobCreateReadDelete(t *testing.T) {
	resource := &datasafev1beta1.DiscoveryJob{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"osok-replay-discoveryjob"},"spec":{"sensitiveDataModelId":"ocid1.sensitivedatamodel.oc1..synthetic","compartmentId":"ocid1.compartment.oc1..synthetic","discoveryType":"ALL","displayName":"osok-replay-discovery-job"}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.discoveryjob.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "datasafe", Resource: "DiscoveryJob",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "discoveryjob_synthetic_crud.yaml"), Host: "https://datasafe.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{
			CreatedBody: createdBody, EmptyCollectionBody: "{\"items\":[]}", PresentCollectionBody: "{\"items\":[" + createdBody + "]}", CreateStatus: 0, DeleteStatus: 0,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	manager := &DiscoveryJobServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newDiscoveryJobRuntimeHooks(manager, sdkClient)
	client := wrapDiscoveryJobGeneratedClient(hooks, defaultDiscoveryJobServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*datasafev1beta1.DiscoveryJob](buildDiscoveryJobGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datasafev1beta1.DiscoveryJob]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *datasafev1beta1.DiscoveryJob) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *datasafev1beta1.DiscoveryJob) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created DiscoveryJob status has no OCI identity: %+v", current.Status)
			}
			return nil
		},
	})
}
