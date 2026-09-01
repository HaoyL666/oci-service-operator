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

const (
	recordedDeploymentParentName = "osok-replay-deployment-parent-v1"
	recordedDeploymentName       = "osok-replay-common-api-deployment-v1"
)

func TestRecordedDeploymentCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	if mode == ocireplay.ModeReplay {
		oldGatewayDelay, oldDeploymentDelay := gatewayRetryInterval, deploymentRetryInterval
		gatewayRetryInterval, deploymentRetryInterval = 0, 0
		t.Cleanup(func() {
			gatewayRetryInterval, deploymentRetryInterval = oldGatewayDelay, oldDeploymentDelay
		})
	}
	gatewaySDK, deploymentSDK, closeSession := openRecordedDeploymentSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close API Gateway Deployment cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	subnetID := "ocid1.subnet.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredDeploymentRecordingEnv(t, "OCI_COMPARTMENT_ID")
		subnetID = requiredDeploymentRecordingEnv(t, "OCI_API_GATEWAY_SUBNET_ID")
	}
	log := loggerutil.OSOKLogger{Logger: logr.Discard()}
	gatewayManager := &GatewayServiceManager{
		CredentialClient: &fakeCredentialClient{}, Log: log, ociClient: gatewaySDK,
	}
	gateway := &apigatewayv1beta1.ApiGateway{
		ObjectMeta: metav1.ObjectMeta{Name: recordedDeploymentParentName, Namespace: "default"},
		Spec: apigatewayv1beta1.ApiGatewaySpec{
			CompartmentId: shared.OCID(compartmentID), DisplayName: recordedDeploymentParentName,
			EndpointType: "PUBLIC", SubnetId: shared.OCID(subnetID),
			TagResources: shared.TagResources{FreeFormTags: map[string]string{"osok-replay": "deployment-parent"}},
		},
	}
	deploymentManager := &DeploymentServiceManager{Log: log, ociClient: deploymentSDK}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	deploymentDeleted, gatewayDeleted := false, false
	var deployment *apigatewayv1beta1.ApiGatewayDeployment
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cleanupCancel()
			if !deploymentDeleted && deployment != nil && deployment.Status.OsokStatus.Ocid != "" {
				_ = ocireplay.Await(cleanupCtx, mode, 30*time.Second, func() (bool, error) {
					return deploymentManager.Delete(cleanupCtx, deployment)
				})
			}
			if !gatewayDeleted && gateway.Status.OsokStatus.Ocid != "" {
				_ = ocireplay.Await(cleanupCtx, mode, 30*time.Second, func() (bool, error) {
					return gatewayManager.Delete(cleanupCtx, gateway)
				})
			}
		})
	}

	if err := awaitGatewayConvergence(ctx, mode, gatewayManager, gateway); err != nil {
		t.Fatal(err)
	}
	deployment = &apigatewayv1beta1.ApiGatewayDeployment{Spec: apigatewayv1beta1.ApiGatewayDeploymentSpec{
		GatewayId:     gateway.Status.OsokStatus.Ocid,
		CompartmentId: shared.OCID(compartmentID),
		DisplayName:   recordedDeploymentName,
		PathPrefix:    "/osok-replay",
		Routes: []apigatewayv1beta1.ApiGatewayRoute{{
			Path: "/hello", Methods: []string{"GET"},
			Backend: apigatewayv1beta1.ApiGatewayRouteBackend{Type: "STOCK_RESPONSE_BACKEND", Status: 200, Body: "hello"},
		}},
		TagResources: shared.TagResources{FreeFormTags: map[string]string{"osok-replay": "create"}},
	}}
	if err := awaitDeploymentConvergence(ctx, mode, deploymentManager, deployment); err != nil {
		t.Fatal(err)
	}
	if deployment.Status.OsokStatus.Ocid == "" {
		t.Fatal("created API Gateway Deployment did not project its OCI identifier")
	}

	deployment.Spec.DisplayName = recordedDeploymentName + "-updated"
	deployment.Spec.Routes[0].Backend.Body = "updated"
	deployment.Spec.FreeFormTags = map[string]string{"osok-replay": "update"}
	if err := awaitDeploymentConvergence(ctx, mode, deploymentManager, deployment); err != nil {
		t.Fatal(err)
	}
	if err := awaitDeploymentObservedUpdate(ctx, mode, deploymentSDK, deployment); err != nil {
		t.Fatal(err)
	}

	if err := ocireplay.Await(ctx, mode, deploymentPollInterval(mode), func() (bool, error) {
		return deploymentManager.Delete(ctx, deployment)
	}); err != nil {
		t.Fatal(err)
	}
	deploymentDeleted = true
	if err := ocireplay.Await(ctx, mode, deploymentPollInterval(mode), func() (bool, error) {
		return gatewayManager.Delete(ctx, gateway)
	}); err != nil {
		t.Fatal(err)
	}
	gatewayDeleted = true
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func openRecordedDeploymentSDK(t *testing.T, mode ocireplay.Mode) (apigatewaysdk.GatewayClient, apigatewaysdk.DeploymentClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "apigateway", Resource: "ApiGatewayDeployment", Operations: []ocireplay.Operation{
		ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete,
	}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	cassettePath := filepath.Join("testdata", "recordings", "deployment_crud.yaml")
	bindings := map[string]string{"subnet": "ocid1.subnet.oc1..replay"}
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		gatewayClient, err := apigatewaysdk.NewGatewayClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		deploymentClient, err := apigatewaysdk.NewDeploymentClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		bindings["subnet"] = requiredDeploymentRecordingEnv(t, "OCI_API_GATEWAY_SUBNET_ID")
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: cassettePath, Metadata: metadata, BaseClient: &gatewayClient.BaseClient, Bindings: bindings, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Attach(&deploymentClient.BaseClient); err != nil {
			t.Fatal(err)
		}
		return gatewayClient, deploymentClient, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: cassettePath, Host: "https://apigateway.us-ashburn-1.oci.oraclecloud.com", BasePath: "20190501", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return apigatewaysdk.GatewayClient{BaseClient: session.BaseClient()}, apigatewaysdk.DeploymentClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitDeploymentConvergence(ctx context.Context, mode ocireplay.Mode, manager *DeploymentServiceManager, resource *apigatewayv1beta1.ApiGatewayDeployment) error {
	return ocireplay.Await(ctx, mode, deploymentPollInterval(mode), func() (bool, error) {
		response, err := manager.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			if response.ShouldRequeue {
				return false, nil
			}
			return false, fmt.Errorf("API Gateway Deployment reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func awaitDeploymentObservedUpdate(ctx context.Context, mode ocireplay.Mode, client apigatewaysdk.DeploymentClient, resource *apigatewayv1beta1.ApiGatewayDeployment) error {
	return ocireplay.Await(ctx, mode, deploymentPollInterval(mode), func() (bool, error) {
		response, err := client.GetDeployment(ctx, apigatewaysdk.GetDeploymentRequest{DeploymentId: common.String(string(resource.Status.OsokStatus.Ocid))})
		if err != nil {
			return false, err
		}
		return response.LifecycleState == apigatewaysdk.DeploymentLifecycleStateActive &&
			response.DisplayName != nil && *response.DisplayName == resource.Spec.DisplayName &&
			response.FreeformTags["osok-replay"] == "update", nil
	})
}

func deploymentPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 30 * time.Second
	}
	return 0
}

func requiredDeploymentRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
