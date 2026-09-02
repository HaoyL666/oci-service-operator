/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package unifiedagentconfiguration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	loggingsdk "github.com/oracle/oci-go-sdk/v65/logging"
	loggingv1beta1 "github.com/oracle/oci-service-operator/api/logging/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedUnifiedAgentConfigurationCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "logging", Resource: "UnifiedAgentConfiguration", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	compartmentID, logID, dynamicGroupID := "ocid1.compartment.oc1..replay", "ocid1.log.oc1..replay", "ocid1.dynamicgroup.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredUnifiedAgentConfigurationEnv(t, "OCI_COMPARTMENT_ID")
		logID = requiredUnifiedAgentConfigurationEnv(t, "OCI_REPLAY_LOG_ID")
		dynamicGroupID = requiredUnifiedAgentConfigurationEnv(t, "OCI_REPLAY_DYNAMIC_GROUP_ID")
	}
	sdkClient, closeSession := ocireplay.OpenLoggingManagementSDK(t, mode, filepath.Join("testdata", "recordings", "unifiedagentconfiguration_crud.yaml"), metadata)
	resource := &loggingv1beta1.UnifiedAgentConfiguration{Spec: loggingv1beta1.UnifiedAgentConfigurationSpec{
		IsEnabled:     true,
		CompartmentId: compartmentID,
		DisplayName:   "osok-replay-unified-agent-config",
		Description:   "OSOK recorded unified agent configuration",
		ServiceConfiguration: loggingv1beta1.UnifiedAgentConfigurationServiceConfiguration{
			ConfigurationType: string(loggingsdk.UnifiedAgentServiceConfigurationTypesLogging),
			Sources: []loggingv1beta1.UnifiedAgentConfigurationServiceConfigurationSource{{
				Name:       "osok-replay-application",
				SourceType: string(loggingsdk.UnifiedAgentLoggingSourceSourceTypeLogTail),
				Paths:      []string{"/var/log/osok-replay.log"},
				Parser:     loggingv1beta1.UnifiedAgentConfigurationServiceConfigurationSourceParser{ParserType: string(loggingsdk.UnifiedAgentParserParserTypeNone)},
			}},
			Destination: loggingv1beta1.UnifiedAgentConfigurationServiceConfigurationDestination{LogObjectId: logID},
		},
		GroupAssociation: loggingv1beta1.UnifiedAgentConfigurationGroupAssociation{GroupList: []string{dynamicGroupID}},
	}}
	manager := &UnifiedAgentConfigurationServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newUnifiedAgentConfigurationRuntimeHooksWithOCIClient(sdkClient)
	applyUnifiedAgentConfigurationRuntimeHooks(manager, &hooks, sdkClient, nil)
	client := defaultUnifiedAgentConfigurationServiceClient{ServiceClient: generatedruntime.NewServiceClient[*loggingv1beta1.UnifiedAgentConfiguration](buildUnifiedAgentConfigurationGeneratedRuntimeConfig(manager, hooks))}
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*loggingv1beta1.UnifiedAgentConfiguration]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *loggingv1beta1.UnifiedAgentConfiguration) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *loggingv1beta1.UnifiedAgentConfiguration) error {
			if current.Status.DisplayName != "osok-replay-unified-agent-config" {
				return fmt.Errorf("created UnifiedAgentConfiguration status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *loggingv1beta1.UnifiedAgentConfiguration) {
			current.Spec.DisplayName = "osok-replay-unified-agent-config-updated"
		},
		ValidateUpdated: func(current *loggingv1beta1.UnifiedAgentConfiguration) error {
			if current.Status.DisplayName != "osok-replay-unified-agent-config-updated" {
				return fmt.Errorf("updated UnifiedAgentConfiguration status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredUnifiedAgentConfigurationEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
