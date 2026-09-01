/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package hostname

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	loadbalancerv1beta1 "github.com/oracle/oci-service-operator/api/loadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedHostnameName = "osok_replay_hostname_v1"

func TestRecordedHostnameCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "loadbalancer", Resource: "Hostname", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	sdkClient, closeSession := ocireplay.OpenLoadBalancerSDK(t, mode, filepath.Join("testdata", "recordings", "hostname_crud.yaml"), metadata)
	loadBalancerID := "ocid1.loadbalancer.oc1..replay"
	if mode == ocireplay.ModeRecord {
		loadBalancerID = requiredHostnameRecordingEnv(t, "OCI_REPLAY_LOAD_BALANCER_ID")
	}
	resource := &loadbalancerv1beta1.Hostname{
		ObjectMeta: metav1.ObjectMeta{Name: "recorded-hostname", Annotations: map[string]string{hostnameLoadBalancerIDAnnotation: loadBalancerID}},
		Spec:       loadbalancerv1beta1.HostnameSpec{Name: recordedHostnameName, Hostname: "create.example.com"},
	}
	client := newHostnameRuntimeClient(sdkClient, loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*loadbalancerv1beta1.Hostname]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second,
		HasIdentity: func(current *loadbalancerv1beta1.Hostname) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *loadbalancerv1beta1.Hostname) error {
			if current.Status.Name != recordedHostnameName || current.Status.Hostname != "create.example.com" {
				return fmt.Errorf("created Hostname status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *loadbalancerv1beta1.Hostname) { current.Spec.Hostname = "update.example.com" },
		ValidateUpdated: func(current *loadbalancerv1beta1.Hostname) error {
			if current.Status.Hostname != current.Spec.Hostname {
				return fmt.Errorf("updated Hostname status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredHostnameRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
