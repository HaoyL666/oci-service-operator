/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package subscription

import (
	"context"
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

func TestRecordedSubscriptionCreatePendingDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "ons", Resource: "Subscription", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	sdkClient, closeSession := ocireplay.OpenNotificationSDK(t, mode, filepath.Join("testdata", "recordings", "subscription_create_pending_delete.yaml"), metadata)
	compartmentID, topicID := "ocid1.compartment.oc1..replay", "ocid1.onstopic.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredSubscriptionRecordingEnv(t, "OCI_COMPARTMENT_ID")
		topicID = requiredSubscriptionRecordingEnv(t, "OCI_REPLAY_ONS_TOPIC_ID")
	}
	resource := &onsv1beta1.Subscription{Spec: onsv1beta1.SubscriptionSpec{
		CompartmentId: compartmentID,
		TopicId:       topicID,
		Protocol:      "CUSTOM_HTTPS",
		Endpoint:      "https://example.invalid/osok-replay",
		FreeformTags:  map[string]string{"osok-replay": "create"},
	}}
	client := newSubscriptionServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := ocireplay.Await(createCtx, mode, 5*time.Second, func() (bool, error) {
		response, err := client.CreateOrUpdate(createCtx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Subscription reconciliation was unsuccessful: %+v", response)
		}
		return resource.Status.Id != "" && resource.Status.LifecycleState == string(onssdk.SubscriptionLifecycleStatePending), nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		return client.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
}

func requiredSubscriptionRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
