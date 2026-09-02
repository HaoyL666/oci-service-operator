/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ruleset

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

func TestRecordedRuleSetCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "loadbalancer", Resource: "RuleSet", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	loadBalancerID := "ocid1.loadbalancer.oc1..replay"
	if mode == ocireplay.ModeRecord {
		loadBalancerID = requiredRuleSetRecordingEnv(t, "OCI_REPLAY_LOAD_BALANCER_ID")
	}
	sdkClient, closeSession := ocireplay.OpenLoadBalancerSDK(t, mode, filepath.Join("testdata", "recordings", "ruleset_crud.yaml"), metadata)
	resource := &lbv1beta1.RuleSet{Spec: lbv1beta1.RuleSetSpec{Name: "osok_replay_rule_set", LoadBalancerId: loadBalancerID, Items: []lbv1beta1.RuleSetItem{ruleSetAddRequestHeaderRule("x-osok-replay", "created")}}}
	hooks := newRuleSetRuntimeHooksWithOCIClient(sdkClient)
	applyRuleSetRuntimeHooks(&hooks)
	manager := &RuleSetServiceManager{}
	client := wrapRuleSetGeneratedClient(hooks, defaultRuleSetServiceClient{ServiceClient: generatedruntime.NewServiceClient[*lbv1beta1.RuleSet](buildRuleSetGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*lbv1beta1.RuleSet]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		RetryError: func(err error) bool {
			message := err.Error()
			return strings.Contains(message, "generated runtime resource not found") ||
				strings.Contains(message, "not found, or you do not have authorization")
		},
		HasIdentity: func(current *lbv1beta1.RuleSet) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *lbv1beta1.RuleSet) error {
			if current.Status.Name != resource.Spec.Name || len(current.Status.Items) != 1 {
				return fmt.Errorf("created RuleSet status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *lbv1beta1.RuleSet) { current.Spec.Items[0].Value = "updated" },
		ValidateUpdated: func(current *lbv1beta1.RuleSet) error {
			if len(current.Status.Items) != 1 || current.Status.Items[0].Value != "updated" {
				return fmt.Errorf("updated RuleSet status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredRuleSetRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
