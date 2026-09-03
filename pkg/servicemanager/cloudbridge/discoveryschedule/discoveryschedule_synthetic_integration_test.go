/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package discoveryschedule

import (
	"context"
	"encoding/json"
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

func TestSyntheticDiscoveryScheduleCreateReadDelete(t *testing.T) {
	resource := &cloudbridgev1beta1.DiscoverySchedule{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"osok-replay-discoveryschedule"},"spec":{"compartmentId":"ocid1.compartment.oc1..synthetic","executionRecurrences":"FREQ=DAILY;INTERVAL=1","displayName":"osok-replay-discovery-schedule"}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.discoveryschedule.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "cloudbridge", Resource: "DiscoverySchedule",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "discoveryschedule_synthetic_crud.yaml"), Host: "https://cloudbridge.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220509", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{
			CreatedBody: createdBody, EmptyCollectionBody: "{\"items\":[]}", PresentCollectionBody: "{\"items\":[" + createdBody + "]}", CreateStatus: 0, DeleteStatus: 0,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := cloudbridgesdk.DiscoveryClient{BaseClient: session.BaseClient()}
	manager := &DiscoveryScheduleServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newDiscoveryScheduleRuntimeHooks(manager, sdkClient)
	client := wrapDiscoveryScheduleGeneratedClient(hooks, defaultDiscoveryScheduleServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*cloudbridgev1beta1.DiscoverySchedule](buildDiscoveryScheduleGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*cloudbridgev1beta1.DiscoverySchedule]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *cloudbridgev1beta1.DiscoverySchedule) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *cloudbridgev1beta1.DiscoverySchedule) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created DiscoverySchedule status has no OCI identity: %+v", current.Status)
			}
			return nil
		},
	})
}
