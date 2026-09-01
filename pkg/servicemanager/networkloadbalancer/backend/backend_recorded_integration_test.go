/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	nlbv1beta1 "github.com/oracle/oci-service-operator/api/networkloadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestRecordedBackendCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "networkloadbalancer", Resource: "Backend", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	parentID, backendSet := "ocid1.networkloadbalancer.oc1..replay", "replay_backend_set"
	if mode == ocireplay.ModeRecord {
		parentID = requiredBackendRecordingEnv(t, "OCI_REPLAY_NETWORK_LOAD_BALANCER_ID")
		backendSet = requiredBackendRecordingEnv(t, "OCI_REPLAY_NLB_BACKEND_SET_NAME")
	}
	sdkClient, closeSession := ocireplay.OpenNetworkLoadBalancerSDK(t, mode, filepath.Join("testdata", "recordings", "backend_crud.yaml"), metadata, map[string]string{"nlb-backend-set": backendSet})
	resource := &nlbv1beta1.Backend{Spec: nlbv1beta1.BackendSpec{NetworkLoadBalancerId: parentID, BackendSetName: backendSet, IpAddress: "10.0.20.200", Port: 8080, Weight: 1}}
	hooks := newBackendRuntimeHooksWithOCIClient(sdkClient)
	applyBackendRuntimeHooks(&hooks, sdkClient, nil)
	manager := &BackendServiceManager{Log: loggerutil.OSOKLogger{}}
	client := wrapBackendGeneratedClient(hooks, defaultBackendServiceClient{ServiceClient: generatedruntime.NewServiceClient[*nlbv1beta1.Backend](buildBackendGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*nlbv1beta1.Backend]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *nlbv1beta1.Backend) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *nlbv1beta1.Backend) error {
			if current.Status.IpAddress != "10.0.20.200" {
				return fmt.Errorf("created Backend status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *nlbv1beta1.Backend) { current.Spec.Weight = 2 },
		ValidateUpdated: func(current *nlbv1beta1.Backend) error {
			if current.Status.Weight != 2 {
				return fmt.Errorf("updated Backend status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredBackendRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
