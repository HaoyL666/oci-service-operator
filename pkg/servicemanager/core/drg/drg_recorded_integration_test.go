/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package drg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	coresdk "github.com/oracle/oci-go-sdk/v65/core"
	corev1beta1 "github.com/oracle/oci-service-operator/api/core/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedDrgName = "osok-replay-common-drg-v1"

func TestRecordedDrgCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedDrgSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close DRG cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredDrgRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &corev1beta1.Drg{
		Spec: corev1beta1.DrgSpec{
			CompartmentId: compartmentID,
			DisplayName:   recordedDrgName,
			FreeformTags:  map[string]string{"osok-replay": "create"},
		},
	}
	client := newRecordedDrgClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted || resource.Status.Id == "" {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 10*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitDrgConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(coresdk.DrgLifecycleStateAvailable) ||
		resource.Status.DisplayName != recordedDrgName {
		t.Fatalf("created DRG status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedDrgName + "-updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitDrgConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated DRG status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, drgPollInterval(mode), func() (bool, error) {
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

func newRecordedDrgClient(sdkClient coresdk.VirtualNetworkClient) DrgServiceClient {
	manager := &DrgServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
	}
	hooks := newDrgDefaultRuntimeHooks(sdkClient)
	applyDrgRuntimeHooks(manager, &hooks)
	appendDrgCreateFallbackRuntimeWrapper(manager, &hooks)
	delegate := defaultDrgServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*corev1beta1.Drg](
			buildDrgGeneratedRuntimeConfig(manager, hooks),
		),
	}
	return wrapDrgGeneratedClient(hooks, delegate)
}

func openRecordedDrgSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (coresdk.VirtualNetworkClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service:  "core",
		Resource: "Drg",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "drg_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := coresdk.NewVirtualNetworkClientWithConfigurationProvider(provider)
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
		Host:     "https://iaas.us-ashburn-1.oraclecloud.com",
		BasePath: "20160918",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return coresdk.VirtualNetworkClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitDrgConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client DrgServiceClient,
	resource *corev1beta1.Drg,
) error {
	return ocireplay.Await(ctx, mode, drgPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("DRG reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func drgPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 10 * time.Second
	}
	return 0
}

func requiredDrgRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
