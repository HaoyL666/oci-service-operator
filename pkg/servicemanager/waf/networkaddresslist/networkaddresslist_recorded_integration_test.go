/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package networkaddresslist

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	wafv1beta1 "github.com/oracle/oci-service-operator/api/waf/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

const recordedNetworkAddressListName = "osok-replay-waf-address-list-v1"

func TestRecordedNetworkAddressListCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "waf", Resource: "NetworkAddressList", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	sdkClient, closeSession := ocireplay.OpenWAFSDK(t, mode, filepath.Join("testdata", "recordings", "networkaddresslist_crud.yaml"), metadata)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredNetworkAddressListRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &wafv1beta1.NetworkAddressList{Spec: wafv1beta1.NetworkAddressListSpec{
		CompartmentId: compartmentID,
		DisplayName:   recordedNetworkAddressListName,
		Type:          networkAddressListTypeAddresses,
		Addresses:     []string{"192.0.2.0/24"},
		FreeformTags:  map[string]string{"osok-replay": "create"},
	}}
	hooks := newNetworkAddressListDefaultRuntimeHooks(sdkClient)
	applyNetworkAddressListRuntimeHooks(&hooks)
	manager := &NetworkAddressListServiceManager{Log: loggerutil.OSOKLogger{}}
	client := wrapNetworkAddressListGeneratedClient(hooks, defaultNetworkAddressListServiceClient{ServiceClient: generatedruntime.NewServiceClient[*wafv1beta1.NetworkAddressList](buildNetworkAddressListGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*wafv1beta1.NetworkAddressList]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *wafv1beta1.NetworkAddressList) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *wafv1beta1.NetworkAddressList) error {
			if current.Status.DisplayName != recordedNetworkAddressListName || len(current.Status.Addresses) != 1 {
				return fmt.Errorf("created NetworkAddressList status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *wafv1beta1.NetworkAddressList) {
			current.Spec.Addresses = []string{"198.51.100.0/24"}
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *wafv1beta1.NetworkAddressList) error {
			if len(current.Status.Addresses) != 1 || current.Status.Addresses[0] != "198.51.100.0/24" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated NetworkAddressList status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredNetworkAddressListRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
