/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package tsigkey

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	dnssdk "github.com/oracle/oci-go-sdk/v65/dns"
	dnsv1beta1 "github.com/oracle/oci-service-operator/api/dns/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedTsigKeyName = "osok-replay-tsig-key-v1"

func TestRecordedTsigKeyCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedTsigKeySDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close TsigKey cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredTsigKeyRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	secret := "<redacted>"
	if mode == ocireplay.ModeRecord {
		secret = "cmVwbGF5LXRzaWcta2V5"
	}
	resource := &dnsv1beta1.TsigKey{Spec: dnsv1beta1.TsigKeySpec{
		Algorithm:     "hmac-sha256",
		Name:          recordedTsigKeyName,
		CompartmentId: compartmentID,
		Secret:        secret,
		FreeformTags:  map[string]string{"osok-replay": "create"},
	}}
	client := newTsigKeyServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: logr.Discard()}, sdkClient)
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

	if err := awaitRecordedTsigKey(generatedruntime.WithSkipExistingBeforeCreate(ctx), mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Name != recordedTsigKeyName || resource.Status.Id == "" {
		t.Fatalf("created TsigKey status = %+v", resource.Status)
	}

	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitRecordedTsigKey(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated TsigKey status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, recordedTsigKeyPollInterval(mode), func() (bool, error) {
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

func openRecordedTsigKeySDK(t *testing.T, mode ocireplay.Mode) (dnssdk.DnsClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "dns", Resource: "TsigKey", Operations: []ocireplay.Operation{
		ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete,
	}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "tsigkey_crud.yaml")
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
			Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path: path, Host: "https://dns.us-ashburn-1.oci.oraclecloud.com", BasePath: "20180115", Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return dnssdk.DnsClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitRecordedTsigKey(ctx context.Context, mode ocireplay.Mode, client TsigKeyServiceClient, resource *dnsv1beta1.TsigKey) error {
	return ocireplay.Await(ctx, mode, recordedTsigKeyPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("TsigKey reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func recordedTsigKeyPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 5 * time.Second
	}
	return 0
}

func requiredTsigKeyRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
