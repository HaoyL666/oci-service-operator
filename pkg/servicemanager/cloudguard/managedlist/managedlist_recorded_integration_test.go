/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package managedlist

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	cloudguardsdk "github.com/oracle/oci-go-sdk/v65/cloudguard"
	cloudguardv1beta1 "github.com/oracle/oci-service-operator/api/cloudguard/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedManagedListName = "osok-replay-managed-list-v1"

func TestRecordedManagedListCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedManagedListSDK(t, mode)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredManagedListRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &cloudguardv1beta1.ManagedList{ObjectMeta: metav1.ObjectMeta{Name: "recorded-managed-list"}, Spec: cloudguardv1beta1.ManagedListSpec{
		CompartmentId: compartmentID, DisplayName: recordedManagedListName, Description: "recorded create",
		ListType: "CIDR_BLOCK", ListItems: []string{"192.0.2.0/24"}, FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	manager := &ManagedListServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newManagedListRuntimeHooks(manager, sdkClient)
	client := wrapManagedListGeneratedClient(hooks, defaultManagedListServiceClient{ServiceClient: generatedruntime.NewServiceClient[*cloudguardv1beta1.ManagedList](buildManagedListGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*cloudguardv1beta1.ManagedList]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *cloudguardv1beta1.ManagedList) bool { return current.Status.Id != "" },
		ValidateCreated: func(current *cloudguardv1beta1.ManagedList) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedManagedListName {
				return fmt.Errorf("created ManagedList status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *cloudguardv1beta1.ManagedList) {
			current.Spec.Description = "recorded update"
			current.Spec.ListItems = []string{"198.51.100.0/24"}
		},
		ValidateUpdated: func(current *cloudguardv1beta1.ManagedList) error {
			if current.Status.Description != "recorded update" || len(current.Status.ListItems) != 1 || current.Status.ListItems[0] != "198.51.100.0/24" {
				return fmt.Errorf("updated ManagedList status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedManagedListSDK(t *testing.T, mode ocireplay.Mode) (cloudguardsdk.CloudGuardClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "cloudguard", Resource: "ManagedList", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "managedlist_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := cloudguardsdk.NewCloudGuardClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://cloudguard-cp-api.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200131", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return cloudguardsdk.CloudGuardClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredManagedListRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
