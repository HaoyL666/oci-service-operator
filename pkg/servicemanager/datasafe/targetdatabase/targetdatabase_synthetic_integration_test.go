/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package targetdatabase

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

func TestSyntheticTargetDatabaseCreateReadDelete(t *testing.T) {
	resource := &datasafev1beta1.TargetDatabase{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"osok-replay-targetdatabase"},"spec":{"compartmentId":"ocid1.compartment.oc1..synthetic","databaseDetails":{"infrastructureType":"AUTONOMOUS_DATABASE","autonomousDatabaseId":"ocid1.autonomousdatabase.oc1..synthetic"},"displayName":"osok-replay-target-database"}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.targetdatabase.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "datasafe", Resource: "TargetDatabase",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "targetdatabase_synthetic_crud.yaml"), Host: "https://datasafe.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{
			CreatedBody: createdBody, EmptyCollectionBody: "[]", PresentCollectionBody: "[" + createdBody + "]", CreateStatus: 0, DeleteStatus: 0,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	manager := &TargetDatabaseServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newTargetDatabaseRuntimeHooks(manager, sdkClient)
	client := wrapTargetDatabaseGeneratedClient(hooks, defaultTargetDatabaseServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*datasafev1beta1.TargetDatabase](buildTargetDatabaseGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datasafev1beta1.TargetDatabase]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *datasafev1beta1.TargetDatabase) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *datasafev1beta1.TargetDatabase) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created TargetDatabase status has no OCI identity: %+v", current.Status)
			}
			return nil
		},
	})
}
