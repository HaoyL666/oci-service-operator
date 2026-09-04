/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package organization

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tenantmanagercontrolplanesdk "github.com/oracle/oci-go-sdk/v65/tenantmanagercontrolplane"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedOrganizationReadsExisting(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	organizationClient, workClient, closeSession := openRecordedOrganizationSDK(t, mode)
	resource := newOrganizationResource()
	resource.Spec.CompartmentId = "ocid1.tenancy.oc1..replay"
	if mode == ocireplay.ModeRecord {
		resource.Spec.CompartmentId = requiredOrganizationRecordingEnv(t, "OCI_TENANCY_ID")
	}
	client := newOrganizationServiceClientWithClients(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, organizationClient, workClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.Id == "" || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("recorded Organization response=%+v status=%+v", response, resource.Status)
	}
	if resource.Status.LifecycleState != string(tenantmanagercontrolplanesdk.OrganizationLifecycleStateActive) {
		t.Fatalf("recorded Organization lifecycleState=%q", resource.Status.LifecycleState)
	}
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
}

func openRecordedOrganizationSDK(t *testing.T, mode ocireplay.Mode) (tenantmanagercontrolplanesdk.OrganizationClient, tenantmanagercontrolplanesdk.WorkRequestClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "tenantmanagercontrolplane", Resource: "Organization", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "organization_read.yaml")
	bindings := map[string]string{"organization-parent-name": "replay-parent"}
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		organizationClient, err := tenantmanagercontrolplanesdk.NewOrganizationClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		workClient, err := tenantmanagercontrolplanesdk.NewWorkRequestClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		bindings["organization-parent-name"] = requiredOrganizationRecordingEnv(t, "OCI_ORGANIZATION_PARENT_NAME")
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &organizationClient.BaseClient, Bindings: bindings, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Attach(&workClient.BaseClient); err != nil {
			t.Fatal(err)
		}
		return organizationClient, workClient, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://organizations.us-ashburn-1.oci.oraclecloud.com", BasePath: "20230401", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	baseClient := session.BaseClient()
	return tenantmanagercontrolplanesdk.OrganizationClient{BaseClient: baseClient}, tenantmanagercontrolplanesdk.WorkRequestClient{BaseClient: baseClient}, session.Close
}

func requiredOrganizationRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
