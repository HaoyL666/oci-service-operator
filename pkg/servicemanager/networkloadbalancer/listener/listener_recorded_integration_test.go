/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package listener

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	nlbv1beta1 "github.com/oracle/oci-service-operator/api/networkloadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestRecordedListenerCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "networkloadbalancer", Resource: "Listener", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	parentID, backendSetName := "ocid1.networkloadbalancer.oc1..replay", "osok_replay_nlb_backend_set"
	if mode == ocireplay.ModeRecord {
		parentID = requiredListenerRecordingEnv(t, "OCI_REPLAY_NETWORK_LOAD_BALANCER_ID")
		backendSetName = requiredListenerRecordingEnv(t, "OCI_REPLAY_NLB_BACKEND_SET_NAME")
	}
	sdkClient, closeSession := ocireplay.OpenNetworkLoadBalancerSDK(t, mode, filepath.Join("testdata", "recordings", "listener_crud.yaml"), metadata, map[string]string{"nlb-listener-backend-set": backendSetName})
	resource := &nlbv1beta1.Listener{Spec: nlbv1beta1.ListenerSpec{Name: "osok_replay_listener", NetworkLoadBalancerId: parentID, DefaultBackendSetName: backendSetName, Port: 9080, Protocol: "TCP"}}
	hooks := newListenerRuntimeHooksWithOCIClient(sdkClient)
	applyListenerRuntimeHooks(&hooks, sdkClient, nil)
	manager := &ListenerServiceManager{}
	client := wrapListenerGeneratedClient(hooks, defaultListenerServiceClient{ServiceClient: generatedruntime.NewServiceClient[*nlbv1beta1.Listener](buildListenerGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*nlbv1beta1.Listener]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		RetryError:    func(err error) bool { return strings.Contains(err.Error(), "generated runtime resource not found") },
		HasIdentity:   func(current *nlbv1beta1.Listener) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *nlbv1beta1.Listener) error {
			if current.Status.Name != resource.Spec.Name || current.Status.Port != 9080 {
				return fmt.Errorf("created Listener status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *nlbv1beta1.Listener) { current.Spec.Port = 9081 },
		ValidateUpdated: func(current *nlbv1beta1.Listener) error {
			if current.Status.Port != 9081 {
				return fmt.Errorf("updated Listener status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredListenerRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
