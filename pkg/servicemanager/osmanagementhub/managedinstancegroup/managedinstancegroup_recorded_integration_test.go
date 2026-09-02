/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package managedinstancegroup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	osmanagementhubv1beta1 "github.com/oracle/oci-service-operator/api/osmanagementhub/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestRecordedManagedInstanceGroupCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "osmanagementhub", Resource: "ManagedInstanceGroup", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	compartmentID := "ocid1.compartment.oc1..replay"
	softwareSourceID := "ocid1.osmhsoftwaresource.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredManagedInstanceGroupEnv(t, "OCI_COMPARTMENT_ID")
		softwareSourceID = requiredManagedInstanceGroupEnv(t, "OCI_REPLAY_OSMH_SOFTWARE_SOURCE_ID")
	}
	sdkClient, closeSession := ocireplay.OpenOSManagementHubManagedInstanceGroupSDK(t, mode, filepath.Join("testdata", "recordings", "managedinstancegroup_crud.yaml"), metadata)
	resource := &osmanagementhubv1beta1.ManagedInstanceGroup{Spec: osmanagementhubv1beta1.ManagedInstanceGroupSpec{DisplayName: "osok-replay-managed-instance-group", CompartmentId: compartmentID, OsFamily: "ORACLE_LINUX_8", ArchType: "X86_64", VendorName: "ORACLE", Description: "recorded create", Location: "OCI_COMPUTE", SoftwareSourceIds: []string{softwareSourceID}}}
	client := newManagedInstanceGroupServiceClientWithOCIClient(loggerutil.OSOKLogger{}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*osmanagementhubv1beta1.ManagedInstanceGroup]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *osmanagementhubv1beta1.ManagedInstanceGroup) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *osmanagementhubv1beta1.ManagedInstanceGroup) error {
			if current.Status.Description != "recorded create" {
				return fmt.Errorf("created status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *osmanagementhubv1beta1.ManagedInstanceGroup) {
			current.Spec.Description = "recorded update"
		},
		ValidateUpdated: func(current *osmanagementhubv1beta1.ManagedInstanceGroup) error {
			if current.Status.Description != "recorded update" {
				return fmt.Errorf("updated status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredManagedInstanceGroupEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
