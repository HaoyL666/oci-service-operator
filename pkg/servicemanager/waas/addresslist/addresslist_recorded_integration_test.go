/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package addresslist

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	waasv1beta1 "github.com/oracle/oci-service-operator/api/waas/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedAddressListName = "osok-replay-waas-address-list-v1"

func TestRecordedAddressListCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "waas", Resource: "AddressList", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	sdkClient, closeSession := ocireplay.OpenWAASSDK(t, mode, filepath.Join("testdata", "recordings", "addresslist_crud.yaml"), metadata)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredAddressListRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &waasv1beta1.AddressList{Spec: waasv1beta1.AddressListSpec{
		CompartmentId: compartmentID,
		DisplayName:   recordedAddressListName,
		Addresses:     []string{"192.0.2.10", "198.51.100.0/24"},
		FreeformTags:  map[string]string{"osok-replay": "create"},
	}}
	hooks := newAddressListDefaultRuntimeHooks(sdkClient)
	applyAddressListRuntimeHooks(&hooks, sdkClient, nil)
	manager := &AddressListServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	client := wrapAddressListGeneratedClient(hooks, defaultAddressListServiceClient{ServiceClient: generatedruntime.NewServiceClient[*waasv1beta1.AddressList](buildAddressListGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*waasv1beta1.AddressList]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *waasv1beta1.AddressList) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *waasv1beta1.AddressList) error {
			if current.Status.DisplayName != recordedAddressListName || len(current.Status.Addresses) != 2 {
				return fmt.Errorf("created AddressList status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *waasv1beta1.AddressList) {
			current.Spec.Addresses = []string{"192.0.2.20"}
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *waasv1beta1.AddressList) error {
			if len(current.Status.Addresses) != 1 || current.Status.Addresses[0] != "192.0.2.20" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated AddressList status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredAddressListRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
