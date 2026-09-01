/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package emaildomain

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	emailsdk "github.com/oracle/oci-go-sdk/v65/email"
	emailv1beta1 "github.com/oracle/oci-service-operator/api/email/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedEmailDomainName = "osok-replay-recorded.example.com"

func TestRecordedEmailDomainCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedEmailDomainSDK(t, mode)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredEmailDomainRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &emailv1beta1.EmailDomain{Spec: emailv1beta1.EmailDomainSpec{
		CompartmentId: compartmentID, Name: recordedEmailDomainName,
		Description: "recorded create", FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	manager := &EmailDomainServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newEmailDomainRuntimeHooks(manager, sdkClient)
	client := wrapEmailDomainGeneratedClient(hooks, defaultEmailDomainServiceClient{ServiceClient: generatedruntime.NewServiceClient[*emailv1beta1.EmailDomain](buildEmailDomainGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*emailv1beta1.EmailDomain]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, Timeout: 30 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *emailv1beta1.EmailDomain) bool { return current.Status.Id != "" },
		ValidateCreated: func(current *emailv1beta1.EmailDomain) error {
			if current.Status.Id == "" || current.Status.Name != recordedEmailDomainName {
				return fmt.Errorf("created EmailDomain status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *emailv1beta1.EmailDomain) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *emailv1beta1.EmailDomain) error {
			if current.Status.Description != current.Spec.Description || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated EmailDomain status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedEmailDomainSDK(t *testing.T, mode ocireplay.Mode) (emailsdk.EmailClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "email", Resource: "EmailDomain", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "emaildomain_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := emailsdk.NewEmailClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://ctrl.email.us-ashburn-1.oci.oraclecloud.com", BasePath: "20170907", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return emailsdk.EmailClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredEmailDomainRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
