/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package backendset

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

func TestRecordedBackendSetCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "networkloadbalancer", Resource: "BackendSet", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	parentID := "ocid1.networkloadbalancer.oc1..replay"
	if mode == ocireplay.ModeRecord {
		parentID = requiredBackendSetRecordingEnv(t, "OCI_REPLAY_NETWORK_LOAD_BALANCER_ID")
	}
	sdkClient, closeSession := ocireplay.OpenNetworkLoadBalancerSDK(t, mode, filepath.Join("testdata", "recordings", "backendset_crud.yaml"), metadata)
	resource := &nlbv1beta1.BackendSet{Spec: nlbv1beta1.BackendSetSpec{
		Name: "osok_replay_backend_set_child", NetworkLoadBalancerId: parentID, Policy: "FIVE_TUPLE", IpVersion: "IPV4",
		HealthChecker: nlbv1beta1.BackendSetHealthChecker{Protocol: "TCP", Port: 8080},
	}}
	hooks := newBackendSetRuntimeHooksWithOCIClient(sdkClient)
	applyBackendSetRuntimeHooks(&hooks, sdkClient, nil)
	manager := &BackendSetServiceManager{}
	client := wrapBackendSetGeneratedClient(hooks, defaultBackendSetServiceClient{ServiceClient: generatedruntime.NewServiceClient[*nlbv1beta1.BackendSet](buildBackendSetGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*nlbv1beta1.BackendSet]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		RetryError:    func(err error) bool { return strings.Contains(err.Error(), "generated runtime resource not found") },
		HasIdentity:   func(current *nlbv1beta1.BackendSet) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *nlbv1beta1.BackendSet) error {
			if current.Status.Name != resource.Spec.Name || current.Status.NetworkLoadBalancerId != parentID {
				return fmt.Errorf("created BackendSet status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *nlbv1beta1.BackendSet) { current.Spec.Policy = "TWO_TUPLE" },
		ValidateUpdated: func(current *nlbv1beta1.BackendSet) error {
			if current.Status.Policy != "TWO_TUPLE" {
				return fmt.Errorf("updated BackendSet status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredBackendSetRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
