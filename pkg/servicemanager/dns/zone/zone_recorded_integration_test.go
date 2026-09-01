/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package zone

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	dnssdk "github.com/oracle/oci-go-sdk/v65/dns"
	dnsv1beta1 "github.com/oracle/oci-service-operator/api/dns/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedPrivateZoneName = "osok-replay-zone-v1.internal"

func TestRecordedPrivateZoneCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedZoneSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close private Zone cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	viewID := "ocid1.dnsview.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredZoneRecordingEnv(t, "OCI_COMPARTMENT_ID")
		viewID = requiredZoneRecordingEnv(t, "OCI_REPLAY_DNS_VIEW_ID")
	}
	resource := &dnsv1beta1.Zone{Spec: dnsv1beta1.ZoneSpec{
		Name:          recordedPrivateZoneName,
		CompartmentId: compartmentID,
		ViewId:        viewID,
		ZoneType:      string(dnssdk.CreateZoneDetailsZoneTypePrimary),
		Scope:         string(dnssdk.ScopePrivate),
		FreeformTags:  map[string]string{"osok-replay": "create"},
	}}
	client := newZoneServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
		nil,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 10*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	if err := awaitZoneConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(dnssdk.ZoneLifecycleStateActive) ||
		resource.Status.Name != recordedPrivateZoneName ||
		resource.Status.Scope != string(dnssdk.ScopePrivate) {
		t.Fatalf("created private Zone status = %+v", resource.Status)
	}

	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitZoneConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated private Zone status = %+v", resource.Status)
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

func openRecordedZoneSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (dnssdk.DnsClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "dns",
		Resource: "Zone",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "zone_crud.yaml")

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

func awaitZoneConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client ZoneServiceClient,
	resource *dnsv1beta1.Zone,
) error {
	return ocireplay.Await(ctx, mode, 10*time.Second, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("private Zone reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredZoneRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
