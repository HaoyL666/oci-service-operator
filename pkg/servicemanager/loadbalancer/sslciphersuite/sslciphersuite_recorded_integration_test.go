/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package sslciphersuite

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	loadbalancerv1beta1 "github.com/oracle/oci-service-operator/api/loadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const recordedSSLCipherSuiteName = "osok_replay_ssl_cipher_v2"

func TestRecordedSSLCipherSuiteCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "loadbalancer", Resource: "SSLCipherSuite", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	sdkClient, closeSession := ocireplay.OpenLoadBalancerSDK(t, mode, filepath.Join("testdata", "recordings", "sslciphersuite_crud.yaml"), metadata)
	loadBalancerID := "ocid1.loadbalancer.oc1..replay"
	if mode == ocireplay.ModeRecord {
		loadBalancerID = requiredSSLCipherSuiteRecordingEnv(t, "OCI_REPLAY_LOAD_BALANCER_ID")
	}
	resource := &loadbalancerv1beta1.SSLCipherSuite{
		ObjectMeta: metav1.ObjectMeta{Name: "recorded-ssl-cipher", Annotations: map[string]string{sslCipherSuiteLoadBalancerIDAnnotation: loadBalancerID}},
		Spec:       loadbalancerv1beta1.SSLCipherSuiteSpec{Name: recordedSSLCipherSuiteName, Ciphers: []string{"ECDHE-RSA-AES256-GCM-SHA384"}},
	}
	hooks := newSSLCipherSuiteRuntimeHooksWithOCIClient(sdkClient)
	applySSLCipherSuiteRuntimeHooks(&hooks, sdkClient, nil, loggerutil.OSOKLogger{})
	delegate := defaultSSLCipherSuiteServiceClient{ServiceClient: generatedruntime.NewServiceClient[*loadbalancerv1beta1.SSLCipherSuite](buildSSLCipherSuiteGeneratedRuntimeConfig(&SSLCipherSuiteServiceManager{}, hooks))}
	client := wrapSSLCipherSuiteGeneratedClient(hooks, delegate)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*loadbalancerv1beta1.SSLCipherSuite]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second,
		HasIdentity: func(current *loadbalancerv1beta1.SSLCipherSuite) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *loadbalancerv1beta1.SSLCipherSuite) error {
			if current.Status.Name != recordedSSLCipherSuiteName || len(current.Status.Ciphers) != 1 {
				return fmt.Errorf("created SSLCipherSuite status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *loadbalancerv1beta1.SSLCipherSuite) {
			current.Spec.Ciphers = append(current.Spec.Ciphers, "ECDHE-RSA-AES128-GCM-SHA256")
		},
		ValidateUpdated: func(current *loadbalancerv1beta1.SSLCipherSuite) error {
			if len(current.Status.Ciphers) != 2 {
				return fmt.Errorf("updated SSLCipherSuite status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredSSLCipherSuiteRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
