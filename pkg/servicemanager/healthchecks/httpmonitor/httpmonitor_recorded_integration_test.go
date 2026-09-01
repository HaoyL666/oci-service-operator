/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package httpmonitor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	healthcheckssdk "github.com/oracle/oci-go-sdk/v65/healthchecks"
	healthchecksv1beta1 "github.com/oracle/oci-service-operator/api/healthchecks/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedHTTPMonitorName = "osok-replay-common-http-monitor-v1"

func TestRecordedHttpMonitorCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedHTTPMonitorSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close HTTP Monitor cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredHTTPMonitorRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &healthchecksv1beta1.HttpMonitor{Spec: healthchecksv1beta1.HttpMonitorSpec{
		CompartmentId:     compartmentID,
		Targets:           []string{"example.com"},
		Protocol:          "HTTPS",
		DisplayName:       recordedHTTPMonitorName,
		IntervalInSeconds: 60,
		Port:              443,
		TimeoutInSeconds:  10,
		Method:            "GET",
		Path:              "/",
		IsEnabled:         false,
		FreeformTags:      map[string]string{"osok-replay": "create"},
	}}
	client := newRecordedHTTPMonitorClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted || resource.Status.Id == "" {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 5*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitHTTPMonitorConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != recordedHTTPMonitorName || resource.Status.IsEnabled {
		t.Fatalf("created HTTP Monitor status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedHTTPMonitorName + "-updated"
	resource.Spec.IntervalInSeconds = 30
	resource.Spec.Path = "/health"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitHTTPMonitorConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.IntervalInSeconds != resource.Spec.IntervalInSeconds ||
		resource.Status.Path != resource.Spec.Path ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated HTTP Monitor status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, httpMonitorPollInterval(mode), func() (bool, error) {
		deleted, err := client.Delete(ctx, resource)
		if err != nil && strings.Contains(err.Error(), "ambiguous 404 NotAuthorizedOrNotFound") {
			return false, nil
		}
		return deleted, err
	}); err != nil {
		t.Fatal(err)
	}
	deleted = true
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func newRecordedHTTPMonitorClient(sdkClient healthcheckssdk.HealthChecksClient) HttpMonitorServiceClient {
	manager := &HttpMonitorServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newHttpMonitorRuntimeHooks(manager, sdkClient)
	delegate := defaultHttpMonitorServiceClient{ServiceClient: generatedruntime.NewServiceClient[*healthchecksv1beta1.HttpMonitor](
		buildHttpMonitorGeneratedRuntimeConfig(manager, hooks),
	)}
	return wrapHttpMonitorGeneratedClient(hooks, delegate)
}

func openRecordedHTTPMonitorSDK(t *testing.T, mode ocireplay.Mode) (healthcheckssdk.HealthChecksClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "healthchecks", Resource: "HttpMonitor", Operations: []ocireplay.Operation{
		ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete,
	}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	cassettePath := filepath.Join("testdata", "recordings", "httpmonitor_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := healthcheckssdk.NewHealthChecksClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: cassettePath, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: cassettePath, Host: "https://healthchecks.us-ashburn-1.oci.oraclecloud.com", BasePath: "20180501", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return healthcheckssdk.HealthChecksClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitHTTPMonitorConvergence(ctx context.Context, mode ocireplay.Mode, client HttpMonitorServiceClient, resource *healthchecksv1beta1.HttpMonitor) error {
	return ocireplay.Await(ctx, mode, httpMonitorPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("HTTP Monitor reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func httpMonitorPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 5 * time.Second
	}
	return 0
}

func requiredHTTPMonitorRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
