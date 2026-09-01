/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package view

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	dnssdk "github.com/oracle/oci-go-sdk/v65/dns"
	dnsv1beta1 "github.com/oracle/oci-service-operator/api/dns/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/errorutil"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedViewName = "osok-replay-dns-view-v1"
const recordedViewPollInterval = 15 * time.Second

func TestRecordedViewCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedViewSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close View cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredViewRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &dnsv1beta1.View{Spec: dnsv1beta1.ViewSpec{
		CompartmentId: compartmentID,
		DisplayName:   recordedViewName,
		FreeformTags:  map[string]string{"osok-replay": "create"},
	}}
	client := newViewServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, recordedViewPollInterval, func() (bool, error) {
				deleted, err := client.Delete(cleanupCtx, resource)
				if viewRecordingRetryable(err) {
					return false, nil
				}
				return deleted, err
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitViewConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(dnssdk.ViewLifecycleStateActive) ||
		resource.Status.DisplayName != recordedViewName || resource.Status.IsProtected {
		t.Fatalf("created View status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedViewName + "-updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitViewConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated View status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, recordedViewPollInterval, func() (bool, error) {
		deleted, err := client.Delete(ctx, resource)
		if viewRecordingRetryable(err) {
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

func openRecordedViewSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (dnssdk.DnsClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "dns",
		Resource: "View",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "view_crud.yaml")

	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := dnssdk.NewDnsClientWithConfigurationProvider(provider)
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
		Host:     "https://dns.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20180115",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return dnssdk.DnsClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitViewConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client ViewServiceClient,
	resource *dnsv1beta1.View,
) error {
	return ocireplay.Await(ctx, mode, recordedViewPollInterval, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			if viewRecordingRetryable(err) {
				return false, nil
			}
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("View reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func viewRecordingRetryable(err error) bool {
	if err == nil {
		return false
	}
	var throttlingErr errorutil.TooManyRequestsOciError
	if errors.As(err, &throttlingErr) {
		return true
	}
	var serviceErr common.ServiceError
	return errors.As(err, &serviceErr) && serviceErr.GetHTTPStatusCode() == 429
}

func requiredViewRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
