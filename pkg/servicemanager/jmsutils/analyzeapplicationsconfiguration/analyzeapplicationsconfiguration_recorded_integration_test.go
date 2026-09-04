/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package analyzeapplicationsconfiguration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	jmsutilssdk "github.com/oracle/oci-go-sdk/v65/jmsutils"
	jmsutilsv1beta1 "github.com/oracle/oci-service-operator/api/jmsutils/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedAnalyzeApplicationsConfigurationReadsTenancySingleton(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, compartmentID, closeSession := openRecordedAnalyzeApplicationsConfigurationSDK(t, mode)
	resource := &jmsutilsv1beta1.AnalyzeApplicationsConfiguration{}
	client := &analyzeApplicationsConfigurationRuntimeClient{
		client:        sdkClient,
		compartmentID: common.String(compartmentID),
	}
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful {
		t.Fatalf("AnalyzeApplicationsConfiguration response = %+v", response)
	}
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
}

func openRecordedAnalyzeApplicationsConfigurationSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (jmsutilssdk.JmsUtilsClient, string, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service:    "jmsutils",
		Resource:   "AnalyzeApplicationsConfiguration",
		Operations: []ocireplay.Operation{ocireplay.OperationRead},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "analyzeapplicationsconfiguration_read.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := jmsutilssdk.NewJmsUtilsClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path:       cassettePath,
			Metadata:   metadata,
			BaseClient: &client.BaseClient,
			Overwrite:  ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		tenancyID := strings.TrimSpace(os.Getenv("OCI_TENANCY_ID"))
		if tenancyID == "" {
			t.Fatal("OCI_TENANCY_ID is required in record mode")
		}
		return client, tenancyID, session.Close
	}

	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path:     cassettePath,
		Host:     "https://javamanagement-utils.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20250521",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return jmsutilssdk.JmsUtilsClient{BaseClient: session.BaseClient()}, "ocid1.tenancy.oc1..replay", session.Close
}
