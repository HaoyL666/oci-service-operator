/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package recoveryservicesubnet

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	recoverysdk "github.com/oracle/oci-go-sdk/v65/recovery"
	recoveryv1beta1 "github.com/oracle/oci-service-operator/api/recovery/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedRecoveryServiceSubnetName = "osok-replay-recovery-subnet"

func TestRecordedRecoveryServiceSubnetCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	vcnID := "ocid1.vcn.oc1..replay"
	subnetID := "ocid1.subnet.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredRecoveryServiceSubnetEnv(t, "OCI_COMPARTMENT_ID")
		vcnID = requiredRecoveryServiceSubnetEnv(t, "OCI_REPLAY_VCN_ID")
		subnetID = requiredRecoveryServiceSubnetEnv(t, "OCI_REPLAY_SUBNET_ID")
	}
	sdkClient, closeSession := openRecordedRecoveryServiceSubnetSDK(t, mode)
	resource := &recoveryv1beta1.RecoveryServiceSubnet{Spec: recoveryv1beta1.RecoveryServiceSubnetSpec{
		DisplayName: recordedRecoveryServiceSubnetName, CompartmentId: compartmentID, VcnId: vcnID, Subnets: []string{subnetID},
		FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	client := newRecoveryServiceSubnetServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*recoveryv1beta1.RecoveryServiceSubnet]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 10 * time.Second, Timeout: 30 * time.Minute, CleanupTimeout: 20 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		RetryError:    func(err error) bool { return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429) },
		HasIdentity: func(current *recoveryv1beta1.RecoveryServiceSubnet) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *recoveryv1beta1.RecoveryServiceSubnet) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedRecoveryServiceSubnetName {
				return fmt.Errorf("created RecoveryServiceSubnet status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *recoveryv1beta1.RecoveryServiceSubnet) {
			current.Spec.DisplayName = recordedRecoveryServiceSubnetName + "-updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *recoveryv1beta1.RecoveryServiceSubnet) error {
			if current.Status.DisplayName != current.Spec.DisplayName || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated RecoveryServiceSubnet status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedRecoveryServiceSubnetSDK(t *testing.T, mode ocireplay.Mode) (recoverysdk.DatabaseRecoveryClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "recovery", Resource: "RecoveryServiceSubnet", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "recoveryservicesubnet_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := recoverysdk.NewDatabaseRecoveryClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://recovery.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210216", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return recoverysdk.DatabaseRecoveryClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredRecoveryServiceSubnetEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
