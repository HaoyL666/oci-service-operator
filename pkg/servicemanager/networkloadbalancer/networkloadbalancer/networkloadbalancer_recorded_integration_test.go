/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package networkloadbalancer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	nlbsdk "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	nlbv1beta1 "github.com/oracle/oci-service-operator/api/networkloadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedNetworkLoadBalancerName = "osok-replay-common-nlb-v1"

func TestRecordedNetworkLoadBalancerCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedNetworkLoadBalancerSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close NetworkLoadBalancer cassette: %v", err)
			}
		}
	})
	compartmentID, subnetID := "ocid1.compartment.oc1..replay", "ocid1.subnet.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredNetworkLoadBalancerRecordingEnv(t, "OCI_COMPARTMENT_ID")
		subnetID = requiredNetworkLoadBalancerRecordingEnv(t, "OCI_REPLAY_SUBNET_ID")
	}
	resource := &nlbv1beta1.NetworkLoadBalancer{Spec: nlbv1beta1.NetworkLoadBalancerSpec{
		CompartmentId: compartmentID,
		DisplayName:   recordedNetworkLoadBalancerName,
		SubnetId:      subnetID,
		IsPrivate:     true,
		FreeformTags:  map[string]string{"osok-replay": "create"},
	}}
	client := newNetworkLoadBalancerServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
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
	if err := awaitNetworkLoadBalancerConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(nlbsdk.LifecycleStateActive) ||
		resource.Status.DisplayName != recordedNetworkLoadBalancerName {
		t.Fatalf("created NetworkLoadBalancer status = %+v", resource.Status)
	}
	resource.Spec.DisplayName = recordedNetworkLoadBalancerName + "-updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitNetworkLoadBalancerConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated NetworkLoadBalancer status = %+v", resource.Status)
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

func openRecordedNetworkLoadBalancerSDK(t *testing.T, mode ocireplay.Mode) (nlbsdk.NetworkLoadBalancerClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "networkloadbalancer", Resource: "NetworkLoadBalancer",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "networkloadbalancer_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := nlbsdk.NewNetworkLoadBalancerClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://network-load-balancer-api.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200501", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return nlbsdk.NetworkLoadBalancerClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitNetworkLoadBalancerConvergence(ctx context.Context, mode ocireplay.Mode, client NetworkLoadBalancerServiceClient, resource *nlbv1beta1.NetworkLoadBalancer) error {
	return ocireplay.Await(ctx, mode, 10*time.Second, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("NetworkLoadBalancer reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredNetworkLoadBalancerRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
