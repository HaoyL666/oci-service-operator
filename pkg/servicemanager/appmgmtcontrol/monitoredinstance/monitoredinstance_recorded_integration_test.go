/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package monitoredinstance

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	appmgmtcontrolsdk "github.com/oracle/oci-go-sdk/v65/appmgmtcontrol"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedMonitoredInstanceRead(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	resource := newMonitoredInstanceResource()
	resource.Spec.CompartmentId = "ocid1.compartment.oc1..replay"
	resource.Spec.DisplayName = "osok-replay-monitored-instance"
	if mode == ocireplay.ModeRecord {
		resource.Spec.CompartmentId = requiredMonitoredInstanceEnv(t, "OCI_COMPARTMENT_ID")
		resource.Spec.DisplayName = requiredMonitoredInstanceEnv(t, "OCI_REPLAY_MONITORED_INSTANCE_NAME")
	}
	sdkClient, closeSession := openRecordedMonitoredInstanceSDK(t, mode, resource.Spec.DisplayName)
	client := newMonitoredInstanceServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.InstanceId == "" || resource.Status.DisplayName != resource.Spec.DisplayName || resource.Status.LifecycleState == "" {
		t.Fatalf("read response=%+v status=%+v", response, resource.Status)
	}
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
}

func openRecordedMonitoredInstanceSDK(t *testing.T, mode ocireplay.Mode, displayName string) (appmgmtcontrolsdk.AppmgmtControlClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "appmgmtcontrol", Resource: "MonitoredInstance",
		Operations: []ocireplay.Operation{ocireplay.OperationRead},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "monitoredinstance_read.yaml")
	bindings := map[string]string{"monitored-instance-name": displayName}
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := appmgmtcontrolsdk.NewAppmgmtControlClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://cp.appmgmt.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210330", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return appmgmtcontrolsdk.AppmgmtControlClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredMonitoredInstanceEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
