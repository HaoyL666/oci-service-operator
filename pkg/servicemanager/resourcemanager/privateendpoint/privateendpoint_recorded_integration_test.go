/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package privateendpoint

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	resourcemanagersdk "github.com/oracle/oci-go-sdk/v65/resourcemanager"
	resourcemanagerv1beta1 "github.com/oracle/oci-service-operator/api/resourcemanager/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedPrivateEndpointName = "osok-replay-resource-manager-private-endpoint-v5"

func TestRecordedPrivateEndpointCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	vcnID := "ocid1.vcn.oc1.iad.replay"
	subnetID := "ocid1.subnet.oc1.iad.replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredPrivateEndpointRecordingEnv(t, "OCI_COMPARTMENT_ID")
		vcnID = requiredPrivateEndpointRecordingEnv(t, "OCI_REPLAY_VCN_ID")
		subnetID = requiredPrivateEndpointRecordingEnv(t, "OCI_REPLAY_SUBNET_ID")
	}
	sdkClient, closeSession := openRecordedPrivateEndpointSDK(t, mode, vcnID, subnetID)
	resource := &resourcemanagerv1beta1.PrivateEndpoint{
		ObjectMeta: metav1.ObjectMeta{Name: "recorded-private-endpoint-v5"},
		Spec: resourcemanagerv1beta1.PrivateEndpointSpec{
			CompartmentId: compartmentID,
			DisplayName:   recordedPrivateEndpointName,
			VcnId:         vcnID,
			SubnetId:      subnetID,
			Description:   "recorded create",
			FreeformTags:  map[string]string{"osok-replay": "create"},
		},
	}
	manager := &PrivateEndpointServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newPrivateEndpointRuntimeHooks(manager, sdkClient)
	client := wrapPrivateEndpointGeneratedClient(hooks, defaultPrivateEndpointServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*resourcemanagerv1beta1.PrivateEndpoint](buildPrivateEndpointGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*resourcemanagerv1beta1.PrivateEndpoint]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 10 * time.Second, Timeout: 30 * time.Minute, CleanupTimeout: 15 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		RetryError: func(err error) bool {
			return ocireplay.IsHTTPStatus(err, 404) || ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429)
		},
		RetryDeleteError: func(err error) bool {
			return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429)
		},
		HasIdentity: func(current *resourcemanagerv1beta1.PrivateEndpoint) bool {
			return current.Status.Id != "" || current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *resourcemanagerv1beta1.PrivateEndpoint) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedPrivateEndpointName || current.Status.VcnId != vcnID || current.Status.SubnetId != subnetID {
				return fmt.Errorf("created PrivateEndpoint status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *resourcemanagerv1beta1.PrivateEndpoint) {
			current.Spec.DisplayName = recordedPrivateEndpointName + "-updated"
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *resourcemanagerv1beta1.PrivateEndpoint) error {
			if current.Status.DisplayName != current.Spec.DisplayName || current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated PrivateEndpoint status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedPrivateEndpointSDK(t *testing.T, mode ocireplay.Mode, vcnID string, subnetID string) (resourcemanagersdk.ResourceManagerClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "resourcemanager", Resource: "PrivateEndpoint",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "privateendpoint_crud.yaml")
	bindings := map[string]string{"vcn-id": vcnID, "subnet-id": subnetID}
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := resourcemanagersdk.NewResourceManagerClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://resourcemanager.us-ashburn-1.oraclecloud.com", BasePath: "20180917", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return resourcemanagersdk.ResourceManagerClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredPrivateEndpointRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
