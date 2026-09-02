/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package script

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	apmsyntheticsv1beta1 "github.com/oracle/oci-service-operator/api/apmsynthetics/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedScriptCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "apmsynthetics", Resource: "Script",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	domainID := "ocid1.apmdomain.oc1..replay"
	if mode == ocireplay.ModeRecord {
		domainID = requiredScriptEnv(t, "OCI_REPLAY_APM_DOMAIN_ID")
	}
	sdkClient, closeSession := ocireplay.OpenAPMSyntheticsSDK(
		t, mode, filepath.Join("testdata", "recordings", "script_crud.yaml"), metadata,
	)
	resource := &apmsyntheticsv1beta1.Script{Spec: apmsyntheticsv1beta1.ScriptSpec{
		DisplayName: "osok-replay-apm-script", ContentType: "SIDE",
		Content: recordedSeleniumScript("OSOK replay"), ContentFileName: "osok-replay.side",
		ApmDomainId: domainID, FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	client := newScriptServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*apmsyntheticsv1beta1.Script]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *apmsyntheticsv1beta1.Script) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *apmsyntheticsv1beta1.Script) error {
			if current.Status.DisplayName != "osok-replay-apm-script" || current.Status.ContentType != "SIDE" {
				return fmt.Errorf("created Script status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *apmsyntheticsv1beta1.Script) {
			current.Spec.DisplayName = "osok-replay-apm-script-updated"
			current.Spec.Content = recordedSeleniumScript("OSOK replay updated")
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *apmsyntheticsv1beta1.Script) error {
			if current.Status.DisplayName != "osok-replay-apm-script-updated" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated Script status = %+v", current.Status)
			}
			return nil
		},
	})
}

func recordedSeleniumScript(name string) string {
	return `{"id":"0d346d5f-b5e0-41b8-a739-d37ecf0f8b31","version":"2.0","name":"` + name + `","url":"https://example.com","tests":[{"id":"e2376b10-696c-4c74-aef0-78c710cedca9","name":"Replay","commands":[{"id":"d8a96cb7-044f-421a-96dd-b8731841e87e","comment":"","command":"open","target":"/","targets":[],"value":""}]}],"suites":[{"id":"84222275-96b0-4f46-a8ee-7300c78f91bd","name":"Default Suite","persistSession":false,"parallel":false,"timeout":300,"tests":["e2376b10-696c-4c74-aef0-78c710cedca9"]}],"urls":["https://example.com"],"plugins":[]}`
}

func requiredScriptEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
