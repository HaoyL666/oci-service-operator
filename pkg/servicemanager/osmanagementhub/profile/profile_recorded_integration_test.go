/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package profile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	osmanagementhubsdk "github.com/oracle/oci-go-sdk/v65/osmanagementhub"
	osmanagementhubv1beta1 "github.com/oracle/oci-service-operator/api/osmanagementhub/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestRecordedProfileCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "osmanagementhub", Resource: "Profile", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	compartmentID, softwareSourceID := "ocid1.compartment.oc1..replay", "ocid1.osmhsoftwaresource.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredProfileRecordingEnv(t, "OCI_COMPARTMENT_ID")
		softwareSourceID = requiredProfileRecordingEnv(t, "OCI_REPLAY_OSMH_SOFTWARE_SOURCE_ID")
	}
	sdkClient, closeSession := ocireplay.OpenOSManagementHubProfileSDK(t, mode, filepath.Join("testdata", "recordings", "profile_crud.yaml"), metadata)
	resourceUID := types.UID("replay-profile")
	if mode == ocireplay.ModeRecord {
		resourceUID = types.UID(fmt.Sprintf("record-profile-%d", time.Now().UnixNano()))
	}
	resource := &osmanagementhubv1beta1.Profile{ObjectMeta: metav1.ObjectMeta{Name: "osok-replay-osmh-profile", Namespace: "default", UID: resourceUID}, Spec: osmanagementhubv1beta1.ProfileSpec{
		DisplayName:       "osok-replay-osmh-profile-v2",
		CompartmentId:     compartmentID,
		Description:       "OSOK recorded OSMH profile",
		ProfileType:       string(osmanagementhubsdk.ProfileTypeSoftwaresource),
		RegistrationType:  string(osmanagementhubsdk.ProfileRegistrationTypeOciLinux),
		VendorName:        string(osmanagementhubsdk.VendorNameOracle),
		OsFamily:          string(osmanagementhubsdk.OsFamilyOracleLinux8),
		ArchType:          string(osmanagementhubsdk.ArchTypeX8664),
		SoftwareSourceIds: []string{softwareSourceID},
		FreeformTags:      map[string]string{"osok-replay": "create"},
	}}
	client := newProfileServiceClientWithOCIClient(sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*osmanagementhubv1beta1.Profile]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *osmanagementhubv1beta1.Profile) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *osmanagementhubv1beta1.Profile) error {
			if current.Status.DisplayName != "osok-replay-osmh-profile-v2" || current.Status.Id == "" {
				return fmt.Errorf("created Profile status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *osmanagementhubv1beta1.Profile) {
			current.Spec.DisplayName = "osok-replay-osmh-profile-v2-updated"
			current.Spec.Description = "OSOK recorded OSMH profile updated"
		},
		ValidateUpdated: func(current *osmanagementhubv1beta1.Profile) error {
			if current.Status.DisplayName != "osok-replay-osmh-profile-v2-updated" || current.Status.Description != "OSOK recorded OSMH profile updated" {
				return fmt.Errorf("updated Profile status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredProfileRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
