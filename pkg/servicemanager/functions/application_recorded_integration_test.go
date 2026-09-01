/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package functions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	ocifunctions "github.com/oracle/oci-go-sdk/v65/functions"
	functionsv1beta1 "github.com/oracle/oci-service-operator/api/functions/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedFunctionsApplicationName = "osok-replay-functions-app-v1"

func TestRecordedApplicationCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedFunctionsApplicationSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Functions Application cassette: %v", err)
			}
		}
	})

	compartmentID, subnetID := "ocid1.compartment.oc1..replay", "ocid1.subnet.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredFunctionsApplicationRecordingEnv(t, "OCI_COMPARTMENT_ID")
		subnetID = requiredFunctionsApplicationRecordingEnv(t, "OCI_REPLAY_SUBNET_ID")
	}

	resource := &functionsv1beta1.Application{
		Spec: functionsv1beta1.ApplicationSpec{
			CompartmentId: compartmentID,
			DisplayName:   recordedFunctionsApplicationName,
			SubnetIds:     []string{subnetID},
			Shape:         string(ocifunctions.ApplicationShapeX86),
			Config:        map[string]string{"REPLAY_PHASE": "create"},
			FreeformTags:  map[string]string{"osok-replay": "create"},
		},
	}
	manager := (&FunctionsApplicationServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
	}).WithClient(sdkClient)
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
			_ = ocireplay.Await(cleanupCtx, mode, 10*time.Second, func() (bool, error) {
				return manager.Delete(cleanupCtx, resource)
			})
		})
	}

	if err := awaitFunctionsApplicationConvergence(ctx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(ocifunctions.ApplicationLifecycleStateActive) ||
		resource.Status.DisplayName != recordedFunctionsApplicationName ||
		resource.Status.Config["REPLAY_PHASE"] != "create" {
		t.Fatalf("created Functions Application status = %+v", resource.Status)
	}

	resource.Spec.Config = map[string]string{"REPLAY_PHASE": "update"}
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitFunctionsApplicationConvergence(ctx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Config["REPLAY_PHASE"] != "update" ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Functions Application status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, 10*time.Second, func() (bool, error) {
		return manager.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}
	deleted = true

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func openRecordedFunctionsApplicationSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (ocifunctions.FunctionsManagementClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "functions",
		Resource: "Application",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "application_crud.yaml")

	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := ocifunctions.NewFunctionsManagementClientWithConfigurationProvider(provider)
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
		Host:     "https://functions.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20181201",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return ocifunctions.FunctionsManagementClient{
		BaseClient: session.BaseClient(),
	}, session.Close
}

func awaitFunctionsApplicationConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	manager *FunctionsApplicationServiceManager,
	resource *functionsv1beta1.Application,
) error {
	return ocireplay.Await(ctx, mode, 10*time.Second, func() (bool, error) {
		response, err := manager.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf(
				"Functions Application reconciliation was unsuccessful: %+v",
				response,
			)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredFunctionsApplicationRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
