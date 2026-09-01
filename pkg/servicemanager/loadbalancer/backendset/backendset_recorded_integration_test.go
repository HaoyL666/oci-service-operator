/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package backendset

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	loadbalancerv1beta1 "github.com/oracle/oci-service-operator/api/loadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

const recordedBackendSetName = "osok_replay_backend_set_v1"

func TestRecordedBackendSetCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "loadbalancer", Resource: "BackendSet", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	sdkClient, closeSession := ocireplay.OpenLoadBalancerSDK(t, mode, filepath.Join("testdata", "recordings", "backendset_crud.yaml"), metadata)
	loadBalancerID := "ocid1.loadbalancer.oc1..replay"
	if mode == ocireplay.ModeRecord {
		loadBalancerID = requiredBackendSetRecordingEnv(t, "OCI_REPLAY_LOAD_BALANCER_ID")
	}
	resource := &loadbalancerv1beta1.BackendSet{Spec: loadbalancerv1beta1.BackendSetSpec{
		LoadBalancerId: loadBalancerID, Name: recordedBackendSetName, Policy: "ROUND_ROBIN",
		HealthChecker: loadbalancerv1beta1.BackendSetHealthChecker{Protocol: "TCP", Port: 8080, Retries: 3, ReturnCode: 200, ResponseBodyRegex: ".*", TimeoutInMillis: 3000, IntervalInMillis: 10000},
		Backends:      []loadbalancerv1beta1.BackendSetBackend{{IpAddress: "10.0.0.3", Port: 8080, Weight: 1}},
	}}
	hooks := newBackendSetRuntimeHooksWithOCIClient(sdkClient)
	applyBackendSetRuntimeHooks(&hooks)
	client := defaultBackendSetServiceClient{ServiceClient: generatedruntime.NewServiceClient[*loadbalancerv1beta1.BackendSet](buildBackendSetGeneratedRuntimeConfig(&BackendSetServiceManager{}, hooks))}
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*loadbalancerv1beta1.BackendSet]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second,
		RetryError:  func(err error) bool { return ocireplay.IsHTTPStatus(err, 404) },
		HasIdentity: func(current *loadbalancerv1beta1.BackendSet) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *loadbalancerv1beta1.BackendSet) error {
			if current.Status.Name != recordedBackendSetName || current.Status.LoadBalancerId != loadBalancerID {
				return fmt.Errorf("created BackendSet status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *loadbalancerv1beta1.BackendSet) { current.Spec.HealthChecker.Port = 8081 },
		ValidateUpdated: func(current *loadbalancerv1beta1.BackendSet) error {
			if current.Status.HealthChecker.Port != 8081 {
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
