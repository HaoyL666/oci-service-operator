/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package lifecycleenvironment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	osmanagementhubv1beta1 "github.com/oracle/oci-service-operator/api/osmanagementhub/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedLifecycleEnvironmentCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "osmanagementhub", Resource: "LifecycleEnvironment", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredLifecycleEnvironmentEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := ocireplay.OpenOSManagementHubLifecycleEnvironmentSDK(t, mode, filepath.Join("testdata", "recordings", "lifecycleenvironment_crud.yaml"), metadata)
	resource := &osmanagementhubv1beta1.LifecycleEnvironment{Spec: osmanagementhubv1beta1.LifecycleEnvironmentSpec{
		CompartmentId: compartmentID, DisplayName: "osok-replay-lifecycle-environment", Description: "OSOK recorded lifecycle environment",
		Stages: []osmanagementhubv1beta1.LifecycleEnvironmentStage{
			{DisplayName: "Development", Rank: 1},
			{DisplayName: "Testing", Rank: 2},
			{DisplayName: "Production", Rank: 3},
		},
		ArchType: "X86_64", OsFamily: "ORACLE_LINUX_8", VendorName: "ORACLE", Location: "OCI_COMPUTE",
	}}
	client := newLifecycleEnvironmentServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*osmanagementhubv1beta1.LifecycleEnvironment]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		RetryError:    func(err error) bool { return strings.Contains(err.Error(), "NotAuthorizedOrNotFound") },
		HasIdentity: func(current *osmanagementhubv1beta1.LifecycleEnvironment) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *osmanagementhubv1beta1.LifecycleEnvironment) error {
			if current.Status.DisplayName != "osok-replay-lifecycle-environment" {
				return fmt.Errorf("created LifecycleEnvironment status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *osmanagementhubv1beta1.LifecycleEnvironment) {
			current.Spec.DisplayName = "osok-replay-lifecycle-environment-updated"
			current.Spec.Description = "OSOK recorded lifecycle environment updated"
		},
		ValidateUpdated: func(current *osmanagementhubv1beta1.LifecycleEnvironment) error {
			if current.Status.DisplayName != "osok-replay-lifecycle-environment-updated" {
				return fmt.Errorf("updated LifecycleEnvironment status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredLifecycleEnvironmentEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
