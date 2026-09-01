/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package nodepool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	containerenginesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
	containerenginev1beta1 "github.com/oracle/oci-service-operator/api/containerengine/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedNodePoolName = "osok-replay-oke-nodepool-v1"

// TestRecordedNodePoolCreateUpdateDelete records one minimal OKE worker. The
// node metadata explicitly disables legacy IMDS endpoints, making IMDSv2-only
// behavior part of the replay contract rather than an external assumption.
func TestRecordedNodePoolCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedNodePoolSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close NodePool cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	clusterID := "ocid1.cluster.oc1..replay"
	nodeSubnetID := "ocid1.subnet.oc1..nodes"
	nodeImageID := "ocid1.image.oc1..oke"
	availabilityDomain := "example:US-ASHBURN-AD-1"
	kubernetesVersion := "v1.36.1"
	nodeShape := "VM.Standard.E3.Flex"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredNodePoolRecordingEnv(t, "OCI_COMPARTMENT_ID")
		clusterID = requiredNodePoolRecordingEnv(t, "OCI_OKE_CLUSTER_ID")
		nodeSubnetID = requiredNodePoolRecordingEnv(t, "OCI_OKE_NODE_SUBNET_ID")
		nodeImageID = requiredNodePoolRecordingEnv(t, "OCI_OKE_NODE_IMAGE_ID")
		availabilityDomain = requiredNodePoolRecordingEnv(t, "OCI_AVAILABILITY_DOMAIN")
		kubernetesVersion = requiredNodePoolRecordingEnv(t, "OCI_OKE_KUBERNETES_VERSION")
		if configuredShape := os.Getenv("OCI_OKE_NODE_SHAPE"); configuredShape != "" {
			nodeShape = configuredShape
		}
	}

	resource := &containerenginev1beta1.NodePool{
		Spec: containerenginev1beta1.NodePoolSpec{
			CompartmentId:     compartmentID,
			ClusterId:         clusterID,
			Name:              recordedNodePoolName,
			NodeShape:         nodeShape,
			KubernetesVersion: kubernetesVersion,
			NodeMetadata: map[string]string{
				"areLegacyImdsEndpointsDisabled": "true",
			},
			NodeSourceDetails: containerenginev1beta1.NodePoolNodeSourceDetails{
				SourceType:          "IMAGE",
				ImageId:             nodeImageID,
				BootVolumeSizeInGBs: 50,
			},
			NodeShapeConfig: containerenginev1beta1.NodePoolNodeShapeConfig{
				Ocpus:       1,
				MemoryInGBs: 16,
			},
			NodeConfigDetails: containerenginev1beta1.NodePoolNodeConfigDetails{
				Size: 1,
				PlacementConfigs: []containerenginev1beta1.NodePoolNodeConfigDetailsPlacementConfig{
					{
						AvailabilityDomain: availabilityDomain,
						SubnetId:           nodeSubnetID,
					},
				},
				NodePoolPodNetworkOptionDetails: containerenginev1beta1.NodePoolNodeConfigDetailsNodePoolPodNetworkOptionDetails{
					CniType:      "OCI_VCN_IP_NATIVE",
					PodSubnetIds: []string{nodeSubnetID},
				},
			},
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	client := newRecordedNodePoolClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted || resource.Status.Id == "" {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 30*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitNodePoolConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(containerenginesdk.NodePoolLifecycleStateActive) ||
		resource.Status.Name != recordedNodePoolName ||
		resource.Status.NodeMetadata["areLegacyImdsEndpointsDisabled"] != "true" {
		t.Fatalf("created NodePool status = %+v", resource.Status)
	}

	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitNodePoolConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.FreeformTags["osok-replay"] != "update" ||
		resource.Status.NodeMetadata["areLegacyImdsEndpointsDisabled"] != "true" {
		t.Fatalf("updated NodePool status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, nodePoolPollInterval(mode), func() (bool, error) {
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

func newRecordedNodePoolClient(
	sdkClient containerenginesdk.ContainerEngineClient,
) NodePoolServiceClient {
	manager := &NodePoolServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
	}
	hooks := newNodePoolDefaultRuntimeHooks(sdkClient)
	applyNodePoolRuntimeHooks(&hooks)
	delegate := defaultNodePoolServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*containerenginev1beta1.NodePool](
			buildNodePoolGeneratedRuntimeConfig(manager, hooks),
		),
	}
	return wrapNodePoolGeneratedClient(hooks, delegate)
}

func openRecordedNodePoolSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (containerenginesdk.ContainerEngineClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "containerengine",
		Resource: "NodePool",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "nodepool_crud.yaml")

	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := containerenginesdk.NewContainerEngineClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path:       cassettePath,
			Metadata:   metadata,
			BaseClient: &client.BaseClient,
			Bindings: map[string]string{
				"availability-domain": requiredNodePoolRecordingEnv(t, "OCI_AVAILABILITY_DOMAIN"),
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
		Host:     "https://containerengine.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20180222",
		Metadata: metadata,
		Bindings: map[string]string{
			"availability-domain": "example:US-ASHBURN-AD-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return containerenginesdk.ContainerEngineClient{
		BaseClient: session.BaseClient(),
	}, session.Close
}

func awaitNodePoolConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client NodePoolServiceClient,
	resource *containerenginev1beta1.NodePool,
) error {
	return ocireplay.Await(ctx, mode, nodePoolPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("NodePool reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func nodePoolPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 30 * time.Second
	}
	return 0
}

func requiredNodePoolRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
