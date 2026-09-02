/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package listener

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	loadbalancerv1beta1 "github.com/oracle/oci-service-operator/api/loadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedListenerCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "loadbalancer", Resource: "Listener", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	loadBalancerID, backendSetName := "ocid1.loadbalancer.oc1..replay", "osok_replay_backend_set"
	if mode == ocireplay.ModeRecord {
		loadBalancerID = requiredListenerRecordingEnv(t, "OCI_REPLAY_LOAD_BALANCER_ID")
		backendSetName = requiredListenerRecordingEnv(t, "OCI_REPLAY_LB_BACKEND_SET_NAME")
	}
	sdkClient, closeSession := ocireplay.OpenLoadBalancerSDK(t, mode, filepath.Join("testdata", "recordings", "listener_crud.yaml"), metadata)
	resource := &loadbalancerv1beta1.Listener{Spec: loadbalancerv1beta1.ListenerSpec{
		LoadBalancerId:        loadBalancerID,
		Name:                  "osok_replay_listener",
		DefaultBackendSetName: backendSetName,
		Port:                  8080,
		Protocol:              "HTTP",
	}}
	client := newGeneratedListenerServiceClient(sdkClient, loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, nil, nil)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*loadbalancerv1beta1.Listener]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		HasIdentity: func(current *loadbalancerv1beta1.Listener) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *loadbalancerv1beta1.Listener) error {
			if current.Status.Name != "osok_replay_listener" || current.Status.Port != 8080 || current.Status.LoadBalancerId == "" {
				return fmt.Errorf("created Listener status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *loadbalancerv1beta1.Listener) { current.Spec.Port = 8081 },
		ValidateUpdated: func(current *loadbalancerv1beta1.Listener) error {
			if current.Status.Port != 8081 {
				return fmt.Errorf("updated Listener status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredListenerRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
