/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package instance

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

const recordedInstanceName = "osok-replay-async-instance-v1"

func TestRecordedInstanceCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedInstanceSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Instance cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	availabilityDomain := "example:AD-1"
	subnetID := "ocid1.subnet.oc1..replay"
	imageID := "ocid1.image.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredInstanceRecordingEnv(t, "OCI_COMPARTMENT_ID")
		availabilityDomain = requiredInstanceRecordingEnv(t, "OCI_AVAILABILITY_DOMAIN")
		subnetID = requiredInstanceRecordingEnv(t, "OCI_REPLAY_SUBNET_ID")
		imageID = requiredInstanceRecordingEnv(t, "OCI_IMAGE_ID")
	}

	resource := &corev1beta1.Instance{
		Spec: corev1beta1.InstanceSpec{
			AvailabilityDomain: availabilityDomain,
			CompartmentId:      compartmentID,
			DisplayName:        recordedInstanceName,
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
			Shape: "VM.Standard.E4.Flex",
			ShapeConfig: corev1beta1.InstanceShapeConfig{
				Ocpus:       1,
				MemoryInGBs: 16,
			},
			SourceDetails: corev1beta1.InstanceSourceDetails{
				SourceType: "image",
				ImageId:    imageID,
			},
			InstanceOptions: corev1beta1.InstanceOptions{
				AreLegacyImdsEndpointsDisabled: true,
			},
			SubnetId: subnetID,
		},
	}
	manager := newInstanceTestManager(sdkClient)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 10*time.Second, func() (bool, error) {
				return manager.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitInstanceConvergence(createCtx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(coresdk.InstanceLifecycleStateRunning) ||
		resource.Status.DisplayName != recordedInstanceName {
		t.Fatalf("created Instance status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedInstanceName + "-updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitInstanceConvergence(ctx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Instance status = %+v", resource.Status)
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

func openRecordedInstanceSDK(t *testing.T, mode ocireplay.Mode) (coresdk.ComputeClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "core",
		Resource: "Instance",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "instance_crud.yaml")

	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := coresdk.NewComputeClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path:       cassettePath,
			Metadata:   metadata,
			BaseClient: &client.BaseClient,
			Bindings: map[string]string{
				"availability-domain": requiredInstanceRecordingEnv(t, "OCI_AVAILABILITY_DOMAIN"),
			},
			Overwrite: ocireplay.RecordingOverwriteRequested(),
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
		Bindings: map[string]string{
			"availability-domain": "example:AD-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return coresdk.ComputeClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitInstanceConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	manager *InstanceServiceManager,
	resource *corev1beta1.Instance,
) error {
	return ocireplay.Await(ctx, mode, 10*time.Second, func() (bool, error) {
		response, err := manager.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Instance reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredInstanceRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
