/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package cluster

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

const recordedClusterName = "osok-replay-oke-cluster-v1"

// TestRecordedClusterCreateUpdateDelete records a bounded BASIC OKE control
// plane lifecycle. It intentionally creates no node pool; worker compute is
// covered independently by the NodePool recorder.
func TestRecordedClusterCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedClusterSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Cluster cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	vcnID := "ocid1.vcn.oc1..replay"
	endpointSubnetID := "ocid1.subnet.oc1..endpoint"
	serviceLBSubnetID := "ocid1.subnet.oc1..service-lb"
	kubernetesVersion := "v1.36.1"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredClusterRecordingEnv(t, "OCI_COMPARTMENT_ID")
		vcnID = requiredClusterRecordingEnv(t, "OCI_OKE_VCN_ID")
		endpointSubnetID = requiredClusterRecordingEnv(t, "OCI_OKE_ENDPOINT_SUBNET_ID")
		serviceLBSubnetID = requiredClusterRecordingEnv(t, "OCI_OKE_SERVICE_LB_SUBNET_ID")
		kubernetesVersion = requiredClusterRecordingEnv(t, "OCI_OKE_KUBERNETES_VERSION")
	}

	resource := &containerenginev1beta1.Cluster{
		Spec: containerenginev1beta1.ClusterSpec{
			Name:              recordedClusterName,
			CompartmentId:     compartmentID,
			VcnId:             vcnID,
			KubernetesVersion: kubernetesVersion,
			EndpointConfig: containerenginev1beta1.ClusterEndpointConfig{
				SubnetId:          endpointSubnetID,
				IsPublicIpEnabled: false,
			},
			Options: containerenginev1beta1.ClusterOptions{
				ServiceLbSubnetIds: []string{serviceLBSubnetID},
			},
			Type: "BASIC_CLUSTER",
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	client := newRecordedClusterClient(sdkClient)
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
	if err := awaitClusterConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(containerenginesdk.ClusterLifecycleStateActive) ||
		resource.Status.Name != recordedClusterName ||
		resource.Status.Type != "BASIC_CLUSTER" {
		t.Fatalf("created Cluster status = %+v", resource.Status)
	}

	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitClusterConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Cluster status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, clusterPollInterval(mode), func() (bool, error) {
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

func newRecordedClusterClient(
	sdkClient containerenginesdk.ContainerEngineClient,
) ClusterServiceClient {
	manager := &ClusterServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
	}
	hooks := newClusterDefaultRuntimeHooks(sdkClient)
	applyClusterRuntimeHooks(manager, &hooks)
	delegate := defaultClusterServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*containerenginev1beta1.Cluster](
			buildClusterGeneratedRuntimeConfig(manager, hooks),
		),
	}
	return wrapClusterGeneratedClient(hooks, delegate)
}

func openRecordedClusterSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (containerenginesdk.ContainerEngineClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "containerengine",
		Resource: "Cluster",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "cluster_crud.yaml")

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
			Overwrite:  ocireplay.RecordingOverwriteRequested(),
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
	})
	if err != nil {
		t.Fatal(err)
	}
	return containerenginesdk.ContainerEngineClient{
		BaseClient: session.BaseClient(),
	}, session.Close
}

func awaitClusterConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client ClusterServiceClient,
	resource *containerenginev1beta1.Cluster,
) error {
	return ocireplay.Await(ctx, mode, clusterPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Cluster reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func clusterPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 30 * time.Second
	}
	return 0
}

func requiredClusterRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
