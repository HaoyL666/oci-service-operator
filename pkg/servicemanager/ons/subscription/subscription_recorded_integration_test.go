/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package subscription

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
	onsv1beta1 "github.com/oracle/oci-service-operator/api/ons/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedSubscriptionCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "ons", Resource: "Subscription", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	sdkClient, closeSession := ocireplay.OpenNotificationSDK(t, mode, filepath.Join("testdata", "recordings", "subscription_crud.yaml"), metadata)
	compartmentID, topicID := "ocid1.compartment.oc1..replay", "ocid1.onstopic.oc1..replay"
	functionID := "ocid1.fnfunc.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredSubscriptionRecordingEnv(t, "OCI_COMPARTMENT_ID")
		topicID = requiredSubscriptionRecordingEnv(t, "OCI_REPLAY_ONS_TOPIC_ID")
		functionID = requiredSubscriptionRecordingEnv(t, "OCI_REPLAY_ONS_FUNCTION_ID")
	}
	resource := &onsv1beta1.Subscription{Spec: onsv1beta1.SubscriptionSpec{
		CompartmentId: compartmentID,
		TopicId:       topicID,
		Protocol:      "ORACLE_FUNCTIONS",
		Endpoint:      functionID,
		FreeformTags:  map[string]string{"osok-replay": "create"},
	}}
	client := newSubscriptionServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*onsv1beta1.Subscription]{
		Mode:          mode,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  5 * time.Second,
		Timeout:       10 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *onsv1beta1.Subscription) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *onsv1beta1.Subscription) error {
			if current.Status.Id == "" ||
				current.Status.LifecycleState != string(onssdk.SubscriptionLifecycleStateActive) ||
				current.Status.TopicId != current.Spec.TopicId ||
				current.Status.Protocol != current.Spec.Protocol ||
				current.Status.Endpoint != current.Spec.Endpoint ||
				current.Status.FreeformTags["osok-replay"] != "create" {
				return fmt.Errorf("created Subscription status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *onsv1beta1.Subscription) {
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *onsv1beta1.Subscription) error {
			if current.Status.LifecycleState != string(onssdk.SubscriptionLifecycleStateActive) ||
				current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated Subscription status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredSubscriptionRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
