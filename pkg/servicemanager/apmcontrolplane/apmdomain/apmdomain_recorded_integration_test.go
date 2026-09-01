/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package apmdomain

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	apmcontrolplanesdk "github.com/oracle/oci-go-sdk/v65/apmcontrolplane"
	apmcontrolplanev1beta1 "github.com/oracle/oci-service-operator/api/apmcontrolplane/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedApmDomainName = "osok-replay-apm-domain-v1"

func TestRecordedApmDomainCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedApmDomainSDK(t, mode)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredApmDomainRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &apmcontrolplanev1beta1.ApmDomain{ObjectMeta: metav1.ObjectMeta{Name: "recorded-apm-domain"}, Spec: apmcontrolplanev1beta1.ApmDomainSpec{
		CompartmentId: compartmentID, DisplayName: recordedApmDomainName, Description: "recorded create",
		FreeformTags: map[string]string{"osok-replay": "create"}, IsFreeTier: false,
	}}
	client := newApmDomainServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*apmcontrolplanev1beta1.ApmDomain]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 30 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *apmcontrolplanev1beta1.ApmDomain) bool { return current.Status.Id != "" },
		ValidateCreated: func(current *apmcontrolplanev1beta1.ApmDomain) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedApmDomainName {
				return fmt.Errorf("created ApmDomain status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *apmcontrolplanev1beta1.ApmDomain) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *apmcontrolplanev1beta1.ApmDomain) error {
			if current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated ApmDomain status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedApmDomainSDK(t *testing.T, mode ocireplay.Mode) (apmcontrolplanesdk.ApmDomainClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "apmcontrolplane", Resource: "ApmDomain", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "apmdomain_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := apmcontrolplanesdk.NewApmDomainClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://apm-cp.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200630", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return apmcontrolplanesdk.ApmDomainClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredApmDomainRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
