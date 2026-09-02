/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package steeringpolicy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	dnsv1beta1 "github.com/oracle/oci-service-operator/api/dns/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestRecordedSteeringPolicyCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "dns", Resource: "SteeringPolicy", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredSteeringPolicyEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := ocireplay.OpenDNSSDK(t, mode, filepath.Join("testdata", "recordings", "steeringpolicy_crud.yaml"), metadata)
	resource := &dnsv1beta1.SteeringPolicy{Spec: dnsv1beta1.SteeringPolicySpec{
		CompartmentId: compartmentID, DisplayName: "osok-replay-steering-policy", Template: "CUSTOM", Ttl: 30,
		Answers: []dnsv1beta1.SteeringPolicyAnswer{{Name: "primary", Rtype: "A", Rdata: "192.0.2.10", Pool: "blue"}},
		Rules:   []dnsv1beta1.SteeringPolicyRule{{RuleType: "FILTER", DefaultAnswerData: []dnsv1beta1.SteeringPolicyRuleDefaultAnswerData{{AnswerCondition: "answer.isDisabled != true", ShouldKeep: true}}}, {RuleType: "LIMIT", DefaultCount: 1}},
	}}
	manager := &SteeringPolicyServiceManager{}
	hooks := newSteeringPolicyRuntimeHooks(manager, sdkClient)
	client := wrapSteeringPolicyGeneratedClient(hooks, defaultSteeringPolicyServiceClient{ServiceClient: generatedruntime.NewServiceClient[*dnsv1beta1.SteeringPolicy](buildSteeringPolicyGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*dnsv1beta1.SteeringPolicy]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *dnsv1beta1.SteeringPolicy) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *dnsv1beta1.SteeringPolicy) error {
			if current.Status.DisplayName != "osok-replay-steering-policy" || current.Status.Ttl != 30 {
				return fmt.Errorf("created SteeringPolicy status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *dnsv1beta1.SteeringPolicy) {
			current.Spec.DisplayName = "osok-replay-steering-policy-updated"
			current.Spec.Ttl = 60
		},
		ValidateUpdated: func(current *dnsv1beta1.SteeringPolicy) error {
			if current.Status.DisplayName != "osok-replay-steering-policy-updated" || current.Status.Ttl != 60 {
				return fmt.Errorf("updated SteeringPolicy status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredSteeringPolicyEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
