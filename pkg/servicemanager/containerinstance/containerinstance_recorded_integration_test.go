/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package containerinstance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	containerinstancessdk "github.com/oracle/oci-go-sdk/v65/containerinstances"
	coresdk "github.com/oracle/oci-go-sdk/v65/core"
	containerinstancesv1beta1 "github.com/oracle/oci-service-operator/api/containerinstances/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedContainerInstanceName = "osok-replay-common-container-instance-v1"

func TestRecordedContainerInstanceCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, vnicClient, closeSession := openRecordedContainerInstanceSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close ContainerInstance cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	availabilityDomain := "example:US-ASHBURN-AD-1"
	subnetID := "ocid1.subnet.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredContainerInstanceRecordingEnv(t, "OCI_COMPARTMENT_ID")
		availabilityDomain = requiredContainerInstanceRecordingEnv(t, "OCI_AVAILABILITY_DOMAIN")
		subnetID = requiredContainerInstanceRecordingEnv(t, "OCI_CONTAINER_INSTANCE_SUBNET_ID")
	}

	resource := &containerinstancesv1beta1.ContainerInstance{
		Spec: containerinstancesv1beta1.ContainerInstanceSpec{
			CompartmentId:      compartmentID,
			AvailabilityDomain: availabilityDomain,
			Shape:              "CI.Standard.E4.Flex",
			ShapeConfig: containerinstancesv1beta1.ContainerInstanceShapeConfig{
				Ocpus:       1,
				MemoryInGBs: 4,
			},
			Containers: []containerinstancesv1beta1.ContainerInstanceContainer{
				{
					ImageUrl:    "docker.io/library/busybox:1.36.1",
					DisplayName: "worker",
					Command:     []string{"/bin/sh", "-c"},
					Arguments:   []string{"sleep 3600"},
				},
			},
			Vnics: []containerinstancesv1beta1.ContainerInstanceVnic{
				{
					SubnetId:           subnetID,
					DisplayName:        "primary",
					IsPublicIpAssigned: false,
				},
			},
			DisplayName:            recordedContainerInstanceName,
			ContainerRestartPolicy: "NEVER",
			FreeformTags:           map[string]string{"osok-replay": "create"},
		},
	}
	manager := &ContainerInstanceServiceManager{
		Log:        loggerutil.OSOKLogger{Logger: logr.Discard()},
		ociClient:  sdkClient,
		vnicClient: vnicClient,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted || resource.Status.Id == "" {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 15*time.Second, func() (bool, error) {
				return manager.Delete(cleanupCtx, resource)
			})
		})
	}

	if err := awaitContainerInstanceConvergence(ctx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(containerinstancessdk.ContainerInstanceLifecycleStateActive) ||
		resource.Status.DisplayName != recordedContainerInstanceName ||
		resource.Status.ContainerCount != 1 {
		t.Fatalf("created ContainerInstance status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedContainerInstanceName + "-updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitContainerInstanceConvergence(ctx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated ContainerInstance status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, containerInstancePollInterval(mode), func() (bool, error) {
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

func openRecordedContainerInstanceSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (containerinstancessdk.ContainerInstanceClient, coresdk.VirtualNetworkClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service:  "containerinstances",
		Resource: "ContainerInstance",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "containerinstance_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := containerinstancessdk.NewContainerInstanceClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		vnicClient, err := coresdk.NewVirtualNetworkClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path:       cassettePath,
			Metadata:   metadata,
			BaseClient: &client.BaseClient,
			Bindings: map[string]string{
				"availability-domain": requiredContainerInstanceRecordingEnv(t, "OCI_AVAILABILITY_DOMAIN"),
			},
			Overwrite: ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Attach(&vnicClient.BaseClient); err != nil {
			t.Fatal(err)
		}
		return client, vnicClient, session.Close
	}

	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path:     cassettePath,
		Host:     "https://compute-containers.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20210415",
		Metadata: metadata,
		Bindings: map[string]string{
			"availability-domain": "example:US-ASHBURN-AD-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	containerBaseClient := session.BaseClient()
	vnicBaseClient := session.BaseClient()
	vnicBaseClient.Host = "https://iaas.us-ashburn-1.oraclecloud.com"
	vnicBaseClient.BasePath = "20160918"
	return containerinstancessdk.ContainerInstanceClient{BaseClient: containerBaseClient},
		coresdk.VirtualNetworkClient{BaseClient: vnicBaseClient},
		session.Close
}

func awaitContainerInstanceConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	manager *ContainerInstanceServiceManager,
	resource *containerinstancesv1beta1.ContainerInstance,
) error {
	return ocireplay.Await(ctx, mode, containerInstancePollInterval(mode), func() (bool, error) {
		response, err := manager.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("ContainerInstance reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func containerInstancePollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 15 * time.Second
	}
	return 0
}

func requiredContainerInstanceRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
