/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package instanceagentplugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	computeinstanceagentsdk "github.com/oracle/oci-go-sdk/v65/computeinstanceagent"
	computeinstanceagentv1beta1 "github.com/oracle/oci-service-operator/api/computeinstanceagent/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedInstanceAgentPluginRead(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	instanceID := "ocid1.instance.oc1..replay"
	compartmentID := "ocid1.compartment.oc1..replay"
	pluginName := "Compute Instance Monitoring"
	if mode == ocireplay.ModeRecord {
		instanceID = requiredInstanceAgentPluginEnv(t, "OCI_REPLAY_INSTANCE_ID")
		compartmentID = requiredInstanceAgentPluginEnv(t, "OCI_COMPARTMENT_ID")
		if configured := os.Getenv("OCI_REPLAY_INSTANCE_AGENT_PLUGIN_NAME"); configured != "" {
			pluginName = configured
		}
	}

	resource := &computeinstanceagentv1beta1.InstanceAgentPlugin{
		Spec: computeinstanceagentv1beta1.InstanceAgentPluginSpec{
			InstanceagentId: instanceID,
			CompartmentId:   compartmentID,
			PluginName:      pluginName,
		},
	}
	sdkClient, closeSession := openRecordedInstanceAgentPluginSDK(t, mode)
	client := newInstanceAgentPluginServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.Name != pluginName || resource.Status.Status == "" {
		t.Fatalf("read response=%+v status=%+v", response, resource.Status)
	}
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
}

func openRecordedInstanceAgentPluginSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (computeinstanceagentsdk.PluginClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "computeinstanceagent", Resource: "InstanceAgentPlugin",
		Operations: []ocireplay.Operation{ocireplay.OperationRead},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "instanceagentplugin_read.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := computeinstanceagentsdk.NewPluginClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://iaas.us-ashburn-1.oraclecloud.com", BasePath: "20180530", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return computeinstanceagentsdk.PluginClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredInstanceAgentPluginEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
