/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package steeringpolicyattachment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dnsv1beta1 "github.com/oracle/oci-service-operator/api/dns/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestRecordedSteeringPolicyAttachmentCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "dns", Resource: "SteeringPolicyAttachment", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	policyID, zoneID := "ocid1.steeringpolicy.oc1..replay", "ocid1.dns-zone.oc1..replay"
	if mode == ocireplay.ModeRecord {
		policyID = requiredSteeringPolicyAttachmentEnv(t, "OCI_REPLAY_STEERING_POLICY_ID")
		zoneID = requiredSteeringPolicyAttachmentEnv(t, "OCI_REPLAY_DNS_ZONE_ID")
	}
	sdkClient, closeSession := ocireplay.OpenDNSSDK(t, mode, filepath.Join("testdata", "recordings", "steeringpolicyattachment_crud.yaml"), metadata)
	resource := &dnsv1beta1.SteeringPolicyAttachment{Spec: dnsv1beta1.SteeringPolicyAttachmentSpec{SteeringPolicyId: policyID, ZoneId: zoneID, DomainName: "app.osok-replay-batch4.invalid", DisplayName: "osok-replay-steering-attachment"}}
	manager := &SteeringPolicyAttachmentServiceManager{}
	hooks := newSteeringPolicyAttachmentDefaultRuntimeHooks(sdkClient)
	applySteeringPolicyAttachmentRuntimeHooks(&hooks, sdkClient, nil)
	client := wrapSteeringPolicyAttachmentGeneratedClient(hooks, defaultSteeringPolicyAttachmentServiceClient{ServiceClient: generatedruntime.NewServiceClient[*dnsv1beta1.SteeringPolicyAttachment](buildSteeringPolicyAttachmentGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*dnsv1beta1.SteeringPolicyAttachment]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		RetryError:    func(err error) bool { return strings.Contains(err.Error(), "NotAuthorizedOrNotFound") },
		HasIdentity:   func(current *dnsv1beta1.SteeringPolicyAttachment) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *dnsv1beta1.SteeringPolicyAttachment) error {
			if current.Status.DisplayName != "osok-replay-steering-attachment" {
				return fmt.Errorf("created SteeringPolicyAttachment status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *dnsv1beta1.SteeringPolicyAttachment) {
			current.Spec.DisplayName = "osok-replay-steering-attachment-updated"
		},
		ValidateUpdated: func(current *dnsv1beta1.SteeringPolicyAttachment) error {
			if current.Status.DisplayName != "osok-replay-steering-attachment-updated" {
				return fmt.Errorf("updated SteeringPolicyAttachment status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredSteeringPolicyAttachmentEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
