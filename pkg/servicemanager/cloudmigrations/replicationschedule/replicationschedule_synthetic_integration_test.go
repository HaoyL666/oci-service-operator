/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package replicationschedule

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	cloudmigrationssdk "github.com/oracle/oci-go-sdk/v65/cloudmigrations"
	cloudmigrationsv1beta1 "github.com/oracle/oci-service-operator/api/cloudmigrations/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticReplicationScheduleCreateReadDelete(t *testing.T) {
	resource := &cloudmigrationsv1beta1.ReplicationSchedule{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"replication-schedule","namespace":"default"},"spec":{"compartmentId":"ocid1.compartment.oc1..replay","executionRecurrences":"FREQ=DAILY;INTERVAL=1","displayName":"osok-replay-schedule"}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.replicationschedule.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "cloudmigrations", Resource: "ReplicationSchedule", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "replicationschedule_synthetic_crud.yaml"), Host: "https://migration.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220919", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := cloudmigrationssdk.MigrationClient{BaseClient: session.BaseClient()}
	manager := &ReplicationScheduleServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newReplicationScheduleRuntimeHooks(manager, sdkClient)
	client := wrapReplicationScheduleGeneratedClient(hooks, defaultReplicationScheduleServiceClient{ServiceClient: generatedruntime.NewServiceClient[*cloudmigrationsv1beta1.ReplicationSchedule](buildReplicationScheduleGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*cloudmigrationsv1beta1.ReplicationSchedule]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *cloudmigrationsv1beta1.ReplicationSchedule) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *cloudmigrationsv1beta1.ReplicationSchedule) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created ReplicationSchedule status = %+v", current.Status)
		}
		return nil
	}})
}
