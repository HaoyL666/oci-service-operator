/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package functions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ocifunctions "github.com/oracle/oci-go-sdk/v65/functions"
	functionsv1beta1 "github.com/oracle/oci-service-operator/api/functions/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/credhelper"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedFunctionsFunctionName = "osok-replay-function-v1"

func TestRecordedFunctionCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	applicationID := "ocid1.fnapp.oc1..replay"
	image := "iad.ocir.io/replay/functions/osok-replay-function:v1"
	endpointID := "replayendpoint"
	if mode == ocireplay.ModeRecord {
		applicationID = requiredFunctionRecordingEnv(t, "OCI_REPLAY_FUNCTION_APPLICATION_ID")
		image = requiredFunctionRecordingEnv(t, "OCI_REPLAY_FUNCTION_IMAGE")
		endpointID = functionEndpointID(applicationID)
	}
	resource := &functionsv1beta1.Function{
		ObjectMeta: metav1.ObjectMeta{Name: recordedFunctionsFunctionName, Namespace: "default", UID: k8stypes.UID("recorded-function-v1")},
		Spec: functionsv1beta1.FunctionSpec{
			DisplayName: recordedFunctionsFunctionName, ApplicationId: applicationID,
			MemoryInMBs: 256, Image: image, TimeoutInSeconds: 30,
			Config:       map[string]string{"REPLAY_PHASE": "create"},
			FreeformTags: map[string]string{"osok-replay": "create"},
		},
	}

	sdkClient, closeSession := openRecordedFunctionSDK(t, mode, image, endpointID)
	closed := false
	manager := (&FunctionsFunctionServiceManager{
		CredentialClient: replayFunctionCredentialClient{},
		Log:              loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
	}).WithClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
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
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			_ = closeSession()
		}
	})

	if err := awaitRecordedFunction(ctx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Id == "" || resource.Status.LifecycleState != string(ocifunctions.FunctionLifecycleStateActive) || resource.Status.ImageDigest == "" {
		t.Fatalf("created Function status = %+v", resource.Status)
	}
	resource.Spec.Config = map[string]string{"REPLAY_PHASE": "update"}
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	resource.Spec.TimeoutInSeconds = 45
	if err := awaitRecordedFunction(ctx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Config["REPLAY_PHASE"] != "update" || resource.Status.TimeoutInSeconds != 45 {
		t.Fatalf("updated Function status = %+v", resource.Status)
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

func awaitRecordedFunction(ctx context.Context, mode ocireplay.Mode, manager *FunctionsFunctionServiceManager, resource *functionsv1beta1.Function) error {
	return ocireplay.Await(ctx, mode, 10*time.Second, func() (bool, error) {
		response, err := manager.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Function reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue && resource.Status.LifecycleState == string(ocifunctions.FunctionLifecycleStateActive), nil
	})
}

func openRecordedFunctionSDK(t *testing.T, mode ocireplay.Mode, image string, endpointID string) (ocifunctions.FunctionsManagementClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "functions", Resource: "Function",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "function_crud.yaml")
	bindings := map[string]string{
		"function-image": image,
		"function-invoke-endpoint": fmt.Sprintf(
			"https://%s.us-ashburn-1.functions.oci.oraclecloud.com",
			endpointID,
		),
		"function-dualstack-endpoint": fmt.Sprintf(
			"https://%s.ds.functions.us-ashburn-1.oci.oraclecloud.com",
			endpointID,
		),
	}
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := ocifunctions.NewFunctionsManagementClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://functions.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return ocifunctions.FunctionsManagementClient{BaseClient: session.BaseClient()}, session.Close
}

func functionEndpointID(applicationID string) string {
	// OCI Functions derives both invocation hosts from the final 11 characters
	// of the application OCID. Bind the complete hosts so the shorter token is
	// never replaced inside the sanitized application OCID itself.
	const endpointIDLength = 11
	trimmed := strings.TrimSpace(applicationID)
	if len(trimmed) <= endpointIDLength {
		return trimmed
	}
	return trimmed[len(trimmed)-endpointIDLength:]
}

func TestFunctionEndpointID(t *testing.T) {
	t.Parallel()
	if got := functionEndpointID("ocid1.fnapp.oc1.iad.example12345678901"); got != "12345678901" {
		t.Fatalf("functionEndpointID() = %q, want final 11 characters", got)
	}
	if got := functionEndpointID("short"); got != "short" {
		t.Fatalf("functionEndpointID(short) = %q, want short", got)
	}
}

func requiredFunctionRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}

type replayFunctionCredentialClient struct{}

func (replayFunctionCredentialClient) CreateSecret(context.Context, string, string, map[string]string, map[string][]byte) (bool, error) {
	return true, nil
}
func (replayFunctionCredentialClient) DeleteSecret(context.Context, string, string) (bool, error) {
	return true, nil
}
func (replayFunctionCredentialClient) GetSecret(context.Context, string, string) (map[string][]byte, error) {
	return nil, nil
}
func (replayFunctionCredentialClient) UpdateSecret(context.Context, string, string, map[string]string, map[string][]byte) (bool, error) {
	return true, nil
}

var _ credhelper.CredentialClient = replayFunctionCredentialClient{}
