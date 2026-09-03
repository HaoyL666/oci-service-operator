/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package monitoredresourcetype

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	stackmonitoringsdk "github.com/oracle/oci-go-sdk/v65/stackmonitoring"
	stackmonitoringv1beta1 "github.com/oracle/oci-service-operator/api/stackmonitoring/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedMonitoredResourceTypeName = "osok_replay_custom_type"

func TestRecordedMonitoredResourceTypeCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredMonitoredResourceTypeEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := openRecordedMonitoredResourceTypeSDK(t, mode)
	resource := &stackmonitoringv1beta1.MonitoredResourceType{Spec: stackmonitoringv1beta1.MonitoredResourceTypeSpec{
		Name: recordedMonitoredResourceTypeName, CompartmentId: compartmentID, DisplayName: "OSOK replay custom type",
		Description: "recorded create", MetricNamespace: "osok_replay", SourceType: string(stackmonitoringsdk.SourceTypeSmRepoOnly),
		ResourceCategory: string(stackmonitoringsdk.ResourceCategoryApplication), FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	manager := &MonitoredResourceTypeServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newMonitoredResourceTypeDefaultRuntimeHooks(sdkClient)
	applyMonitoredResourceTypeRuntimeHooks(&hooks)
	client := wrapMonitoredResourceTypeGeneratedClient(hooks, defaultMonitoredResourceTypeServiceClient{ServiceClient: generatedruntime.NewServiceClient[*stackmonitoringv1beta1.MonitoredResourceType](buildMonitoredResourceTypeGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*stackmonitoringv1beta1.MonitoredResourceType]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		RetryError:    func(err error) bool { return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429) },
		HasIdentity: func(current *stackmonitoringv1beta1.MonitoredResourceType) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *stackmonitoringv1beta1.MonitoredResourceType) error {
			if current.Status.Id == "" || current.Status.Name != recordedMonitoredResourceTypeName {
				return fmt.Errorf("created MonitoredResourceType status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *stackmonitoringv1beta1.MonitoredResourceType) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *stackmonitoringv1beta1.MonitoredResourceType) error {
			if current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated MonitoredResourceType status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedMonitoredResourceTypeSDK(t *testing.T, mode ocireplay.Mode) (stackmonitoringsdk.StackMonitoringClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "stackmonitoring", Resource: "MonitoredResourceType", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "monitoredresourcetype_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := stackmonitoringsdk.NewStackMonitoringClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://stack-monitoring.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210330", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return stackmonitoringsdk.StackMonitoringClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredMonitoredResourceTypeEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
