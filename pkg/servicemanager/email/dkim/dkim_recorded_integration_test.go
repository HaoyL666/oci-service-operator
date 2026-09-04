/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package dkim

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

const recordedDkimName = "osok-replay-dkim-v1"

func TestRecordedDkimCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	resource := makeDkimResource()
	resource.Spec.EmailDomainId = "ocid1.emaildomain.oc1..replay"
	resource.Spec.Name = recordedDkimName
	resource.Spec.Description = "recorded create"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "create"}
	resource.Spec.DefinedTags = nil
	if mode == ocireplay.ModeRecord {
		resource.Spec.EmailDomainId = requiredDkimRecordingEnv(t, "OCI_REPLAY_EMAIL_DOMAIN_ID")
	}

	sdkClient, closeSession := openRecordedDkimSDK(t, mode)
	manager := &DkimServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newDkimRuntimeHooks(manager, sdkClient)
	client := wrapDkimGeneratedClient(hooks, defaultDkimServiceClient{ServiceClient: generatedruntime.NewServiceClient[*emailv1beta1.Dkim](buildDkimGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*emailv1beta1.Dkim]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 15 * time.Second, Timeout: 90 * time.Minute, CleanupTimeout: 90 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *emailv1beta1.Dkim) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *emailv1beta1.Dkim) error {
			if current.Status.Id == "" || current.Status.Name != recordedDkimName {
				return fmt.Errorf("created DKIM status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *emailv1beta1.Dkim) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *emailv1beta1.Dkim) error {
			if current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated DKIM status = %+v", current.Status)
			}
			return nil
		},
		RetryError:       func(err error) bool { return ocireplay.IsHTTPStatus(err, 429) },
		RetryDeleteError: func(err error) bool { return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429) },
	})
}

func openRecordedDkimSDK(t *testing.T, mode ocireplay.Mode) (emailsdk.EmailClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "email", Resource: "Dkim",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "dkim_crud.yaml")
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

func requiredDkimRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
