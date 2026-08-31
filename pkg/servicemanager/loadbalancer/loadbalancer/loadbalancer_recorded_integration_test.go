/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package loadbalancer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	loadbalancersdk "github.com/oracle/oci-go-sdk/v65/loadbalancer"
	loadbalancerv1beta1 "github.com/oracle/oci-service-operator/api/loadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedLoadBalancerName = "osok-replay-async-lb-v1"

func TestRecordedLoadBalancerCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedLoadBalancerSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close LoadBalancer cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	subnetID := "ocid1.subnet.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredLoadBalancerRecordingEnv(t, "OCI_COMPARTMENT_ID")
		subnetID = requiredLoadBalancerRecordingEnv(t, "OCI_REPLAY_SUBNET_ID")
	}

	resource := &loadbalancerv1beta1.LoadBalancer{
		Spec: loadbalancerv1beta1.LoadBalancerSpec{
			CompartmentId: compartmentID,
			DisplayName:   recordedLoadBalancerName,
			ShapeName:     "flexible",
			SubnetIds:     []string{subnetID},
			ShapeDetails: loadbalancerv1beta1.LoadBalancerShapeDetails{
				MinimumBandwidthInMbps: 10,
				MaximumBandwidthInMbps: 10,
			},
			IsPrivate: true,
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	client := newGeneratedLoadBalancerServiceClient(
		sdkClient,
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		nil,
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
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
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitLoadBalancerConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(loadbalancersdk.LoadBalancerLifecycleStateActive) ||
		resource.Status.DisplayName != recordedLoadBalancerName {
		t.Fatalf("created LoadBalancer status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedLoadBalancerName + "-updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitLoadBalancerConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated LoadBalancer status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, 10*time.Second, func() (bool, error) {
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

func openRecordedLoadBalancerSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (loadbalancersdk.LoadBalancerClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "loadbalancer",
		Resource: "LoadBalancer",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "loadbalancer_crud.yaml")

	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := loadbalancersdk.NewLoadBalancerClientWithConfigurationProvider(provider)
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
		BasePath: "20170115",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return loadbalancersdk.LoadBalancerClient{
		BaseClient: session.BaseClient(),
	}, session.Close
}

func awaitLoadBalancerConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client LoadBalancerServiceClient,
	resource *loadbalancerv1beta1.LoadBalancer,
) error {
	return ocireplay.Await(ctx, mode, 10*time.Second, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("LoadBalancer reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredLoadBalancerRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
