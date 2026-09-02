/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package certificate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	loadbalancerv1beta1 "github.com/oracle/oci-service-operator/api/loadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedCertificateCreateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "loadbalancer", Resource: "Certificate", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	loadBalancerID := "ocid1.loadbalancer.oc1..replay"
	publicCertificate, privateKey := "replay-public-certificate", "replay-private-key"
	if mode == ocireplay.ModeRecord {
		loadBalancerID = requiredCertificateRecordingEnv(t, "OCI_REPLAY_LOAD_BALANCER_ID")
		publicCertificate = strings.TrimSpace(requiredCertificateRecordingFile(t, "OCI_REPLAY_LB_PUBLIC_CERTIFICATE_FILE"))
		privateKey = requiredCertificateRecordingFile(t, "OCI_REPLAY_LB_PRIVATE_KEY_FILE")
	}
	sdkClient, closeSession := ocireplay.OpenLoadBalancerSDKWithBindings(t, mode, filepath.Join("testdata", "recordings", "certificate_crud.yaml"), metadata, map[string]string{"public-certificate": publicCertificate})
	resource := &loadbalancerv1beta1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "osok-replay-lb-certificate",
			Namespace:   "default",
			Annotations: map[string]string{CertificateLoadBalancerIDAnnotation: loadBalancerID},
		},
		Spec: loadbalancerv1beta1.CertificateSpec{
			CertificateName:   "osok_replay_certificate",
			PrivateKey:        privateKey,
			PublicCertificate: publicCertificate,
		},
	}
	client := newCertificateServiceClientWithOCIClient(sdkClient, loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, nil)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*loadbalancerv1beta1.Certificate]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		HasIdentity: func(current *loadbalancerv1beta1.Certificate) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *loadbalancerv1beta1.Certificate) error {
			if current.Status.CertificateName != "osok_replay_certificate" || normalizedRecordedCertificate(current.Status.PublicCertificate) != normalizedRecordedCertificate(resource.Spec.PublicCertificate) {
				return fmt.Errorf("created Certificate status = %+v", current.Status)
			}
			return nil
		},
	})
}

func normalizedRecordedCertificate(value string) string {
	return strings.Join(strings.Fields(value), "")
}

func requiredCertificateRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}

func requiredCertificateRecordingFile(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(requiredCertificateRecordingEnv(t, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}
