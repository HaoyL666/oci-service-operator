/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package vcn

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
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedVcnName = "osok-replay-basic-vcn-v1"

func TestRecordedVcnCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedVcnSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close VCN cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredVcnRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &corev1beta1.Vcn{
		Spec: corev1beta1.VcnSpec{
			CompartmentId: compartmentID,
			CidrBlocks:    []string{"10.95.0.0/16"},
			DisplayName:   recordedVcnName,
			DnsLabel:      "replayvcn",
			FreeformTags:  map[string]string{"osok-replay": "create"},
		},
	}
	manager := newTestManager(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
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
				return manager.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitVcnConvergence(createCtx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(coresdk.VcnLifecycleStateAvailable) || resource.Status.DisplayName != recordedVcnName {
		t.Fatalf("created VCN status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedVcnName + "-updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitVcnConvergence(ctx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName || resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated VCN status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
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

func openRecordedVcnSDK(t *testing.T, mode ocireplay.Mode) (coresdk.VirtualNetworkClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service:    "core",
		Resource:   "Vcn",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "vcn_crud.yaml")
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
			Path:       path,
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
		Path:     path,
		Host:     "https://iaas.us-ashburn-1.oraclecloud.com",
		BasePath: "20160918",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return coresdk.VirtualNetworkClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitVcnConvergence(ctx context.Context, mode ocireplay.Mode, manager *VcnServiceManager, resource *corev1beta1.Vcn) error {
	return ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		response, err := manager.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("VCN reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredVcnRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
