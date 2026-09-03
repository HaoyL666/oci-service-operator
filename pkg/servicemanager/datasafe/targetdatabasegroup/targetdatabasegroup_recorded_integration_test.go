/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package targetdatabasegroup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	datasafesdk "github.com/oracle/oci-go-sdk/v65/datasafe"
	datasafev1beta1 "github.com/oracle/oci-service-operator/api/datasafe/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedTargetDatabaseGroupName = "osok-replay-target-database-group-v1"

func TestRecordedTargetDatabaseGroupCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedTargetDatabaseGroupSDK(t, mode)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredTargetDatabaseGroupRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &datasafev1beta1.TargetDatabaseGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "recorded-target-database-group-v1"},
		Spec: datasafev1beta1.TargetDatabaseGroupSpec{
			CompartmentId: compartmentID,
			DisplayName:   recordedTargetDatabaseGroupName,
			Description:   "recorded create",
			MatchingCriteria: datasafev1beta1.TargetDatabaseGroupMatchingCriteria{
				Include: datasafev1beta1.TargetDatabaseGroupMatchingCriteriaInclude{
					Compartments: []datasafev1beta1.TargetDatabaseGroupMatchingCriteriaIncludeCompartment{{Id: compartmentID}},
				},
			},
			FreeformTags: map[string]string{"osok-replay": "create"},
		},
	}
	manager := &TargetDatabaseGroupServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newTargetDatabaseGroupDefaultRuntimeHooks(sdkClient)
	applyTargetDatabaseGroupRuntimeHooks(&hooks, sdkClient, nil)
	client := wrapTargetDatabaseGroupGeneratedClient(hooks, defaultTargetDatabaseGroupServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*datasafev1beta1.TargetDatabaseGroup](buildTargetDatabaseGroupGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datasafev1beta1.TargetDatabaseGroup]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, Timeout: 20 * time.Minute, CleanupTimeout: 10 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		RetryError:    func(err error) bool { return ocireplay.IsHTTPStatus(err, 409) },
		HasIdentity:   func(current *datasafev1beta1.TargetDatabaseGroup) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *datasafev1beta1.TargetDatabaseGroup) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.DisplayName != recordedTargetDatabaseGroupName {
				return fmt.Errorf("created TargetDatabaseGroup status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *datasafev1beta1.TargetDatabaseGroup) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *datasafev1beta1.TargetDatabaseGroup) error {
			if current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated TargetDatabaseGroup status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedTargetDatabaseGroupSDK(t *testing.T, mode ocireplay.Mode) (datasafesdk.DataSafeClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "datasafe", Resource: "TargetDatabaseGroup",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "targetdatabasegroup_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := datasafesdk.NewDataSafeClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path: path, Host: "https://datasafe.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredTargetDatabaseGroupRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
