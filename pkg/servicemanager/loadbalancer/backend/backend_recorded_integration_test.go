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
	"strings"
	"testing"
	"time"

	lbv1beta1 "github.com/oracle/oci-service-operator/api/loadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestRecordedBackendCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "loadbalancer", Resource: "Backend", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	loadBalancerID, backendSetName := "ocid1.loadbalancer.oc1..replay", "osok_replay_backend_set"
	if mode == ocireplay.ModeRecord {
		loadBalancerID = requiredBackendRecordingEnv(t, "OCI_REPLAY_LOAD_BALANCER_ID")
		backendSetName = requiredBackendRecordingEnv(t, "OCI_REPLAY_LB_BACKEND_SET_NAME")
	}
	sdkClient, closeSession := ocireplay.OpenLoadBalancerSDK(t, mode, filepath.Join("testdata", "recordings", "backend_crud.yaml"), metadata)
	resource := &lbv1beta1.Backend{Spec: lbv1beta1.BackendSpec{LoadBalancerId: loadBalancerID, BackendSetName: backendSetName, IpAddress: "10.0.20.201", Port: 8081, Weight: 1}}
	hooks := newBackendRuntimeHooksWithOCIClient(sdkClient)
	applyBackendRuntimeHooks(&hooks)
	manager := &BackendServiceManager{}
	client := wrapBackendGeneratedClient(hooks, defaultBackendServiceClient{ServiceClient: generatedruntime.NewServiceClient[*lbv1beta1.Backend](buildBackendGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*lbv1beta1.Backend]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		RetryError: func(err error) bool {
			message := err.Error()
			return strings.Contains(message, "generated runtime resource not found")
		},
		HasIdentity: func(current *lbv1beta1.Backend) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *lbv1beta1.Backend) error {
			if current.Status.Name != "10.0.20.201:8081" {
				return fmt.Errorf("created Backend status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *lbv1beta1.Backend) { current.Spec.Weight = 2 },
		ValidateUpdated: func(current *lbv1beta1.Backend) error {
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
