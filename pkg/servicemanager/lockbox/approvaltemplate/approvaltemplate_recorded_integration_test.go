/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package approvaltemplate

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	lockboxsdk "github.com/oracle/oci-go-sdk/v65/lockbox"
	lockboxv1beta1 "github.com/oracle/oci-service-operator/api/lockbox/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedApprovalTemplateName = "osok-replay-approval-template-v1"

func TestRecordedApprovalTemplateCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredApprovalTemplateRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := openRecordedApprovalTemplateSDK(t, mode)
	resource := &lockboxv1beta1.ApprovalTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "recorded-approval-template-v1"},
		Spec: lockboxv1beta1.ApprovalTemplateSpec{
			CompartmentId:     compartmentID,
			DisplayName:       recordedApprovalTemplateName,
			AutoApprovalState: string(lockboxsdk.LockboxAutoApprovalStateEnabled),
			FreeformTags:      map[string]string{"osok-replay": "create"},
		},
	}
	manager := &ApprovalTemplateServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newApprovalTemplateRuntimeHooks(manager, sdkClient)
	client := wrapApprovalTemplateGeneratedClient(hooks, defaultApprovalTemplateServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*lockboxv1beta1.ApprovalTemplate](buildApprovalTemplateGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*lockboxv1beta1.ApprovalTemplate]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, Timeout: 20 * time.Minute, CleanupTimeout: 10 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		RetryError:    func(err error) bool { return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429) },
		HasIdentity: func(current *lockboxv1beta1.ApprovalTemplate) bool {
			return current.Status.Id != "" || current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *lockboxv1beta1.ApprovalTemplate) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedApprovalTemplateName || current.Status.AutoApprovalState != string(lockboxsdk.LockboxAutoApprovalStateEnabled) {
				return fmt.Errorf("created ApprovalTemplate status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *lockboxv1beta1.ApprovalTemplate) {
			current.Spec.DisplayName = recordedApprovalTemplateName + "-updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *lockboxv1beta1.ApprovalTemplate) error {
			if current.Status.DisplayName != current.Spec.DisplayName || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated ApprovalTemplate status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedApprovalTemplateSDK(t *testing.T, mode ocireplay.Mode) (lockboxsdk.LockboxClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "lockbox", Resource: "ApprovalTemplate",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "approvaltemplate_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := lockboxsdk.NewLockboxClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://managed-access.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220126", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return lockboxsdk.LockboxClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredApprovalTemplateRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
