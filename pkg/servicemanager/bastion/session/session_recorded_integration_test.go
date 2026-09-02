/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	bastionsdk "github.com/oracle/oci-go-sdk/v65/bastion"
	bastionv1beta1 "github.com/oracle/oci-service-operator/api/bastion/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedSessionPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIF+41fa69AexhQVkrvqibhTb317Sd3K/i6j/ARj1cMNS osok-replay"

func TestRecordedSessionCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "bastion", Resource: "Session",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	bastionID := "ocid1.bastion.oc1..replay"
	if mode == ocireplay.ModeRecord {
		bastionID = requiredSessionEnv(t, "OCI_REPLAY_BASTION_ID")
	}
	sdkClient, closeSession := ocireplay.OpenBastionSDK(
		t, mode, filepath.Join("testdata", "recordings", "session_crud.yaml"), metadata,
	)
	resource := &bastionv1beta1.Session{Spec: bastionv1beta1.SessionSpec{
		BastionId: bastionID,
		TargetResourceDetails: bastionv1beta1.SessionTargetResourceDetails{
			SessionType:                    "PORT_FORWARDING",
			TargetResourcePrivateIpAddress: "10.99.1.10",
			TargetResourcePort:             22,
		},
		KeyDetails:          bastionv1beta1.SessionKeyDetails{PublicKeyContent: recordedSessionPublicKey},
		DisplayName:         "osok-replay-port-forwarding-session",
		KeyType:             "PUB",
		SessionTtlInSeconds: 1800,
	}}
	client := newSessionServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*bastionv1beta1.Session]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *bastionv1beta1.Session) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *bastionv1beta1.Session) error {
			if current.Status.DisplayName != "osok-replay-port-forwarding-session" || current.Status.LifecycleState != string(bastionsdk.SessionLifecycleStateActive) {
				return fmt.Errorf("created Session status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *bastionv1beta1.Session) {
			current.Spec.DisplayName = "osok-replay-port-forwarding-session-updated"
		},
		ValidateUpdated: func(current *bastionv1beta1.Session) error {
			if current.Status.DisplayName != "osok-replay-port-forwarding-session-updated" {
				return fmt.Errorf("updated Session status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredSessionEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
