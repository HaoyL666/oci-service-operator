/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package apigateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	apigatewaysdk "github.com/oracle/oci-go-sdk/v65/apigateway"
	"github.com/oracle/oci-go-sdk/v65/common"
	apigatewayv1beta1 "github.com/oracle/oci-service-operator/api/apigateway/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedGatewayName = "osok-replay-common-api-gateway-v1"

func TestRecordedGatewayCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	if mode == ocireplay.ModeReplay {
		previousRetryInterval := gatewayRetryInterval
		gatewayRetryInterval = 0
		t.Cleanup(func() { gatewayRetryInterval = previousRetryInterval })
	}
	sdkClient, closeSession := openRecordedGatewaySDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close API Gateway cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	subnetID := "ocid1.subnet.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredGatewayRecordingEnv(t, "OCI_COMPARTMENT_ID")
		subnetID = requiredGatewayRecordingEnv(t, "OCI_API_GATEWAY_SUBNET_ID")
	}
	resource := &apigatewayv1beta1.ApiGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      recordedGatewayName,
			Namespace: "default",
		},
		Spec: apigatewayv1beta1.ApiGatewaySpec{
			CompartmentId: shared.OCID(compartmentID),
			DisplayName:   recordedGatewayName,
			EndpointType:  "PUBLIC",
			SubnetId:      shared.OCID(subnetID),
			TagResources: shared.TagResources{
				FreeFormTags: map[string]string{"osok-replay": "create"},
			},
		},
	}
	manager := &GatewayServiceManager{
		CredentialClient: &fakeCredentialClient{},
		Log:              loggerutil.OSOKLogger{Logger: logr.Discard()},
		ociClient:        sdkClient,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted || resource.Status.OsokStatus.Ocid == "" {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 30*time.Second, func() (bool, error) {
				return manager.Delete(cleanupCtx, resource)
			})
		})
	}

	if err := awaitGatewayConvergence(ctx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.OsokStatus.Ocid == "" {
		t.Fatal("created API Gateway did not project its OCI identifier")
	}

	resource.Spec.DisplayName = recordedGatewayName + "-updated"
	resource.Spec.FreeFormTags = map[string]string{"osok-replay": "update"}
	if err := awaitGatewayConvergence(ctx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if err := awaitGatewayObservedUpdate(ctx, mode, sdkClient, resource); err != nil {
		t.Fatal(err)
	}

	if err := ocireplay.Await(ctx, mode, gatewayPollInterval(mode), func() (bool, error) {
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

func openRecordedGatewaySDK(
	t *testing.T,
	mode ocireplay.Mode,
) (apigatewaysdk.GatewayClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service:  "apigateway",
		Resource: "ApiGateway",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "gateway_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := apigatewaysdk.NewGatewayClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path:       cassettePath,
			Metadata:   metadata,
			BaseClient: &client.BaseClient,
			Bindings: map[string]string{
				"subnet": requiredGatewayRecordingEnv(t, "OCI_API_GATEWAY_SUBNET_ID"),
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
		Host:     "https://apigateway.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20190501",
		Metadata: metadata,
		Bindings: map[string]string{
			"subnet": "ocid1.subnet.oc1..replay",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return apigatewaysdk.GatewayClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitGatewayConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	manager *GatewayServiceManager,
	resource *apigatewayv1beta1.ApiGateway,
) error {
	return ocireplay.Await(ctx, mode, gatewayPollInterval(mode), func() (bool, error) {
		response, err := manager.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			if response.ShouldRequeue {
				return false, nil
			}
			return false, fmt.Errorf("API Gateway reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func awaitGatewayObservedUpdate(
	ctx context.Context,
	mode ocireplay.Mode,
	client apigatewaysdk.GatewayClient,
	resource *apigatewayv1beta1.ApiGateway,
) error {
	return ocireplay.Await(ctx, mode, gatewayPollInterval(mode), func() (bool, error) {
		response, err := client.GetGateway(ctx, apigatewaysdk.GetGatewayRequest{
			GatewayId: common.String(string(resource.Status.OsokStatus.Ocid)),
		})
		if err != nil {
			return false, err
		}
		return response.LifecycleState == apigatewaysdk.GatewayLifecycleStateActive &&
			response.DisplayName != nil && *response.DisplayName == resource.Spec.DisplayName &&
			response.FreeformTags["osok-replay"] == "update", nil
	})
}

func gatewayPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 30 * time.Second
	}
	return 0
}

func requiredGatewayRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
