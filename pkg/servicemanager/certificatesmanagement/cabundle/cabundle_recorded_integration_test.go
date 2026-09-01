/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package cabundle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	certificatesmanagementsdk "github.com/oracle/oci-go-sdk/v65/certificatesmanagement"
	certificatesmanagementv1beta1 "github.com/oracle/oci-service-operator/api/certificatesmanagement/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	recordedCaBundleName = "osok-replay-common-ca-bundle-v1"
	recordedCaBundlePEM  = `-----BEGIN CERTIFICATE-----
MIICuDCCAaACCQCswlehgd76VjANBgkqhkiG9w0BAQsFADAeMRwwGgYDVQQDDBNv
c29rLXJlcGxheS5pbnZhbGlkMB4XDTI2MDkwMTA1NDgyNVoXDTM2MDgyOTA1NDgy
NVowHjEcMBoGA1UEAwwTb3Nvay1yZXBsYXkuaW52YWxpZDCCASIwDQYJKoZIhvcN
AQEBBQADggEPADCCAQoCggEBALzZjMNUR5SvwVaOFTw8eDJSMcdapOu7D+MQw2MS
A+1YAF5OQ3gwoMPmTLrGBte9hxWv2N/gYyM5tyDhoBsRKbHPrDz03Z5AEz/riGWp
YwpoTP/zcQ1SQzbV5p/+O2MvbyzuVZ5kixuFZh78PrjBDkcMGNH0uA0NmdH9lKii
ROOFd0aKMCA5L/dn8ESLzUekNghcKuUAu2Gc61J0v+0/3LOe7aPH0v28ossf2ACu
TmLoIGirLaOJynksSpQIG0SVA93oc2CyltFxLinx6Ec0NKb/7qUyqwRYJatSpQcK
GUFPBDs0+reOI4n7tuyhvBBiXrrqbkMJc8WyPs8ClXJlzb0CAwEAATANBgkqhkiG
9w0BAQsFAAOCAQEALCaJLZARLaB9PpH4JURzQV9Xe/3r+2aWjuneSILxxUTvyJ61
j0i59UVbmCaA8hZXt7QhU3D3K8mJoTBMGAdvYSjLm+scYB8V6MbvJ5kHA7bGIvZL
T+qcCWe0ElyYnvoLpqbki9JJI/zgcXYDY5BwypLhpNykxPQeGjqg88TszrRi7o1C
SXIaSw21TuZVNm42tZPkWBqKWPECONg1i9pkoIDZVNaUt3z4nP6LpRAMrNWHoJQW
fovQtpvBhD2r6F7PHqfkymKTMs+76niaMQV13iQKEFLgaaYbkVYk52eUmjMGAGoL
3TftBbW3NhwflEB5+PAD/jpg5EvXLoQINFdW+w==
-----END CERTIFICATE-----`
)

func TestRecordedCaBundleCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedCaBundleSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close CA Bundle cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredCaBundleRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &certificatesmanagementv1beta1.CaBundle{Spec: certificatesmanagementv1beta1.CaBundleSpec{
		Name:          recordedCaBundleName,
		CompartmentId: compartmentID,
		CaBundlePem:   recordedCaBundlePEM,
		Description:   "recorded create",
		FreeformTags:  map[string]string{"osok-replay": "create"},
	}}
	client := newCaBundleServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted || resource.Status.Id == "" {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 10*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitCaBundleConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(certificatesmanagementsdk.CaBundleLifecycleStateActive) ||
		resource.Status.Name != recordedCaBundleName {
		t.Fatalf("created CA Bundle status = %+v", resource.Status)
	}

	resource.Spec.Description = "recorded update"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitCaBundleConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Description != resource.Spec.Description ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated CA Bundle status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, caBundlePollInterval(mode), func() (bool, error) {
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

func openRecordedCaBundleSDK(t *testing.T, mode ocireplay.Mode) (certificatesmanagementsdk.CertificatesManagementClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "certificatesmanagement", Resource: "CaBundle", Operations: []ocireplay.Operation{
		ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete,
	}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	cassettePath := filepath.Join("testdata", "recordings", "cabundle_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := certificatesmanagementsdk.NewCertificatesManagementClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: cassettePath, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: cassettePath, Host: "https://certificatesmanagement.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210224", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return certificatesmanagementsdk.CertificatesManagementClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitCaBundleConvergence(ctx context.Context, mode ocireplay.Mode, client CaBundleServiceClient, resource *certificatesmanagementv1beta1.CaBundle) error {
	return ocireplay.Await(ctx, mode, caBundlePollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("CA Bundle reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func caBundlePollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 10 * time.Second
	}
	return 0
}

func requiredCaBundleRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
