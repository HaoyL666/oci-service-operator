/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package table

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	nosqlsdk "github.com/oracle/oci-go-sdk/v65/nosql"
	nosqlv1beta1 "github.com/oracle/oci-service-operator/api/nosql/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/errorutil"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedTableName = "osok_replay_common_table_v1"
const recordedTablePollInterval = 65 * time.Second

func TestRecordedTableCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedTableSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Table cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredTableRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &nosqlv1beta1.Table{
		Spec: nosqlv1beta1.TableSpec{
			Name:          recordedTableName,
			CompartmentId: compartmentID,
			DdlStatement: "CREATE TABLE " + recordedTableName +
				" (id INTEGER, payload STRING, PRIMARY KEY(id))",
			TableLimits: nosqlv1beta1.TableLimits{
				MaxReadUnits:    10,
				MaxWriteUnits:   10,
				MaxStorageInGBs: 1,
				CapacityMode:    "PROVISIONED",
			},
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	client := newTableServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()
	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, recordedTablePollInterval, func() (bool, error) {
				deleted, err := client.Delete(cleanupCtx, resource)
				if tableRecordingRetryable(err) {
					return false, nil
				}
				return deleted, err
			})
		})
	}

	if err := awaitTableConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(nosqlsdk.TableLifecycleStateActive) ||
		resource.Status.Name != recordedTableName ||
		resource.Status.TableLimits.MaxReadUnits != 10 {
		t.Fatalf("created Table status = %+v", resource.Status)
	}

	resource.Spec.TableLimits.MaxReadUnits = 20
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitTableConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.TableLimits.MaxReadUnits != 20 ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Table status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, recordedTablePollInterval, func() (bool, error) {
		deleted, err := client.Delete(ctx, resource)
		if tableRecordingRetryable(err) {
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

func openRecordedTableSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (nosqlsdk.NosqlClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "nosql",
		Resource: "Table",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "table_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := nosqlsdk.NewNosqlClientWithConfigurationProvider(provider)
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
		Host:     "https://nosql.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20190828",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return nosqlsdk.NosqlClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitTableConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client TableServiceClient,
	resource *nosqlv1beta1.Table,
) error {
	return ocireplay.Await(ctx, mode, recordedTablePollInterval, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			if tableRecordingRetryable(err) {
				return false, nil
			}
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Table reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func tableRecordingRetryable(err error) bool {
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

func requiredTableRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
