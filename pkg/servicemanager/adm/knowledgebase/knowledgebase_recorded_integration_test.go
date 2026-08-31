/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package knowledgebase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	admsdk "github.com/oracle/oci-go-sdk/v65/adm"
	admv1beta1 "github.com/oracle/oci-service-operator/api/adm/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedKnowledgeBaseName = "osok-replay-async-adm-v1"

func TestRecordedKnowledgeBaseCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedKnowledgeBaseSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close KnowledgeBase cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredKnowledgeBaseRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}

	resource := &admv1beta1.KnowledgeBase{
		Spec: admv1beta1.KnowledgeBaseSpec{
			CompartmentId: compartmentID,
			DisplayName:   recordedKnowledgeBaseName,
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	client := newKnowledgeBaseServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 5*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitKnowledgeBaseConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(admsdk.KnowledgeBaseLifecycleStateActive) ||
		resource.Status.DisplayName != recordedKnowledgeBaseName {
		t.Fatalf("created KnowledgeBase status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedKnowledgeBaseName + "-updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitKnowledgeBaseConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated KnowledgeBase status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		return client.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}
	deleted = true

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func openRecordedKnowledgeBaseSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (admsdk.ApplicationDependencyManagementClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "adm",
		Resource: "KnowledgeBase",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "knowledgebase_crud.yaml")

	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := admsdk.NewApplicationDependencyManagementClientWithConfigurationProvider(provider)
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
		return client, session.Close
	}

	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path:     cassettePath,
		Host:     "https://adm.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20220421",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return admsdk.ApplicationDependencyManagementClient{
		BaseClient: session.BaseClient(),
	}, session.Close
}

func awaitKnowledgeBaseConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client KnowledgeBaseServiceClient,
	resource *admv1beta1.KnowledgeBase,
) error {
	return ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("KnowledgeBase reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredKnowledgeBaseRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
