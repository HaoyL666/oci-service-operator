/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package attributeset

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

const recordedAttributeSetName = "osok-replay-attribute-set-v2"

func TestRecordedAttributeSetCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredAttributeSetRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := openRecordedAttributeSetSDK(t, mode)
	resource := &datasafev1beta1.AttributeSet{
		ObjectMeta: metav1.ObjectMeta{Name: "recorded-attribute-set-v2"},
		Spec: datasafev1beta1.AttributeSetSpec{
			CompartmentId:      compartmentID,
			DisplayName:        recordedAttributeSetName,
			AttributeSetType:   "DATABASE_USER",
			AttributeSetValues: []string{"OSOK_REPLAY_USER"},
			Description:        "recorded create",
			FreeformTags:       map[string]string{"osok-replay": "create"},
		},
	}
	manager := &AttributeSetServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newAttributeSetDefaultRuntimeHooks(sdkClient)
	applyAttributeSetRuntimeHooks(manager, &hooks, sdkClient, nil)
	client := wrapAttributeSetGeneratedClient(hooks, defaultAttributeSetServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*datasafev1beta1.AttributeSet](buildAttributeSetGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datasafev1beta1.AttributeSet]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, Timeout: 20 * time.Minute, CleanupTimeout: 10 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		RetryError:    func(err error) bool { return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429) },
		HasIdentity:   func(current *datasafev1beta1.AttributeSet) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *datasafev1beta1.AttributeSet) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.DisplayName != recordedAttributeSetName || current.Status.AttributeSetType != "DATABASE_USER" {
				return fmt.Errorf("created AttributeSet status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *datasafev1beta1.AttributeSet) {
			current.Spec.AttributeSetValues = []string{"OSOK_REPLAY_USER", "OSOK_REPLAY_USER_2"}
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *datasafev1beta1.AttributeSet) error {
			if current.Status.Description != "recorded update" || len(current.Status.AttributeSetValues) != 2 || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated AttributeSet status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedAttributeSetSDK(t *testing.T, mode ocireplay.Mode) (datasafesdk.DataSafeClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "datasafe", Resource: "AttributeSet",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "attributeset_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := datasafesdk.NewDataSafeClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://datasafe.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredAttributeSetRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
