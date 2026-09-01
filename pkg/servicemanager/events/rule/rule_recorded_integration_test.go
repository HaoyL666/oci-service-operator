/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package rule

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	eventssdk "github.com/oracle/oci-go-sdk/v65/events"
	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
	eventsv1beta1 "github.com/oracle/oci-service-operator/api/events/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	recordedRuleName  = "osok-replay-common-events-rule-v1"
	recordedRuleTopic = "osok-replay-events-rule-topic-v1"
)

func TestRecordedRuleCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	eventsClient, onsClient, closeSession := openRecordedRuleSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Events Rule cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredRuleRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	topicID, err := createRecordedRuleTopic(ctx, mode, onsClient, compartmentID)
	if err != nil {
		t.Fatal(err)
	}
	topicDeleted := false
	t.Cleanup(func() {
		if topicDeleted || mode != ocireplay.ModeRecord {
			return
		}
		_ = deleteRecordedRuleTopic(context.Background(), mode, onsClient, topicID)
	})

	resource := &eventsv1beta1.Rule{Spec: eventsv1beta1.RuleSpec{
		DisplayName:   recordedRuleName,
		IsEnabled:     false,
		Condition:     "{}",
		CompartmentId: compartmentID,
		Actions: eventsv1beta1.RuleActions{Actions: []eventsv1beta1.RuleActionsAction{{
			Description: "disabled test topic action", IsEnabled: false,
			ActionType: "ONS", TopicId: topicID,
		}}},
		Description:  "recorded create",
		FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	client := newRecordedRuleClient(eventsClient)
	ruleDeleted := false
	t.Cleanup(func() {
		if ruleDeleted || mode != ocireplay.ModeRecord || resource.Status.Id == "" {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cleanupCancel()
		_ = ocireplay.Await(cleanupCtx, mode, 5*time.Second, func() (bool, error) {
			return client.Delete(cleanupCtx, resource)
		})
	})

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitRuleConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(eventssdk.RuleLifecycleStateInactive) ||
		resource.Status.DisplayName != recordedRuleName {
		t.Fatalf("created Events Rule status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedRuleName + "-updated"
	resource.Spec.Description = "recorded update"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitRuleConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.Description != resource.Spec.Description ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Events Rule status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, rulePollInterval(mode), func() (bool, error) {
		return client.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}
	ruleDeleted = true
	if err := deleteRecordedRuleTopic(ctx, mode, onsClient, topicID); err != nil {
		t.Fatal(err)
	}
	topicDeleted = true
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func newRecordedRuleClient(sdkClient eventssdk.EventsClient) RuleServiceClient {
	manager := &RuleServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newRuleRuntimeHooks(manager, sdkClient)
	delegate := defaultRuleServiceClient{ServiceClient: generatedruntime.NewServiceClient[*eventsv1beta1.Rule](buildRuleGeneratedRuntimeConfig(manager, hooks))}
	return wrapRuleGeneratedClient(hooks, delegate)
}

func createRecordedRuleTopic(ctx context.Context, mode ocireplay.Mode, client onssdk.NotificationControlPlaneClient, compartmentID string) (string, error) {
	response, err := client.CreateTopic(ctx, onssdk.CreateTopicRequest{CreateTopicDetails: onssdk.CreateTopicDetails{
		CompartmentId: common.String(compartmentID), Name: common.String(recordedRuleTopic), Description: common.String("Events Rule recorder parent"),
	}})
	if err != nil {
		return "", err
	}
	topicID := stringPointerValue(response.TopicId)
	err = ocireplay.Await(ctx, mode, rulePollInterval(mode), func() (bool, error) {
		_, err := client.GetTopic(ctx, onssdk.GetTopicRequest{TopicId: common.String(topicID)})
		return err == nil, err
	})
	return topicID, err
}

func deleteRecordedRuleTopic(ctx context.Context, mode ocireplay.Mode, client onssdk.NotificationControlPlaneClient, topicID string) error {
	_, err := client.DeleteTopic(ctx, onssdk.DeleteTopicRequest{TopicId: common.String(topicID)})
	if err != nil && !recordedRuleNotFound(err) {
		return err
	}
	return ocireplay.Await(ctx, mode, rulePollInterval(mode), func() (bool, error) {
		_, err := client.GetTopic(ctx, onssdk.GetTopicRequest{TopicId: common.String(topicID)})
		if recordedRuleNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

func recordedRuleNotFound(err error) bool {
	if err == nil {
		return false
	}
	serviceErr, ok := common.IsServiceError(err)
	return ok && serviceErr.GetHTTPStatusCode() == 404
}

func openRecordedRuleSDK(t *testing.T, mode ocireplay.Mode) (eventssdk.EventsClient, onssdk.NotificationControlPlaneClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "events", Resource: "Rule", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	cassettePath := filepath.Join("testdata", "recordings", "rule_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		eventsClient, err := eventssdk.NewEventsClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		onsClient, err := onssdk.NewNotificationControlPlaneClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: cassettePath, Metadata: metadata, BaseClient: &eventsClient.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Attach(&onsClient.BaseClient); err != nil {
			t.Fatal(err)
		}
		return eventsClient, onsClient, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: cassettePath, Host: "https://events.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	onsBase := session.BaseClient()
	onsBase.Host = "https://notification.us-ashburn-1.oraclecloud.com"
	return eventssdk.EventsClient{BaseClient: session.BaseClient()}, onssdk.NotificationControlPlaneClient{BaseClient: onsBase}, session.Close
}

func awaitRuleConvergence(ctx context.Context, mode ocireplay.Mode, client RuleServiceClient, resource *eventsv1beta1.Rule) error {
	return ocireplay.Await(ctx, mode, rulePollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Events Rule reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func rulePollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 5 * time.Second
	}
	return 0
}

func requiredRuleRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
