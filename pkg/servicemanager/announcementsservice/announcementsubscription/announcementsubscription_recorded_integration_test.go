/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package announcementsubscription

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	announcementsservicev1beta1 "github.com/oracle/oci-service-operator/api/announcementsservice/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedAnnouncementSubscriptionCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "announcementsservice",
		Resource: "AnnouncementSubscription",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	topicID := "ocid1.onstopic.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredAnnouncementSubscriptionEnv(t, "OCI_COMPARTMENT_ID")
		topicID = requiredAnnouncementSubscriptionEnv(t, "OCI_REPLAY_ANNOUNCEMENT_TOPIC_ID")
	}
	sdkClient, closeSession := ocireplay.OpenAnnouncementSubscriptionSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "announcementsubscription_crud.yaml"),
		metadata,
	)
	manager := &AnnouncementSubscriptionServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
	}
	hooks := newAnnouncementSubscriptionRuntimeHooks(manager, sdkClient)
	client := wrapAnnouncementSubscriptionGeneratedClient(
		hooks,
		defaultAnnouncementSubscriptionServiceClient{
			ServiceClient: generatedruntime.NewServiceClient[*announcementsservicev1beta1.AnnouncementSubscription](
				buildAnnouncementSubscriptionGeneratedRuntimeConfig(manager, hooks),
			),
		},
	)
	resource := &announcementsservicev1beta1.AnnouncementSubscription{
		Spec: announcementsservicev1beta1.AnnouncementSubscriptionSpec{
			CompartmentId:     compartmentID,
			OnsTopicId:        topicID,
			DisplayName:       "osok-replay-announcement-subscription",
			Description:       "OSOK recorded announcement subscription",
			PreferredTimeZone: "UTC",
			FreeformTags:      map[string]string{"osok-replay": "create"},
		},
	}
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*announcementsservicev1beta1.AnnouncementSubscription]{
		Mode:          mode,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  5 * time.Second,
		Timeout:       15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *announcementsservicev1beta1.AnnouncementSubscription) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *announcementsservicev1beta1.AnnouncementSubscription) error {
			if current.Status.DisplayName != "osok-replay-announcement-subscription" {
				return fmt.Errorf("created AnnouncementSubscription status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *announcementsservicev1beta1.AnnouncementSubscription) {
			current.Spec.Description = "OSOK recorded announcement subscription updated"
			current.Spec.PreferredTimeZone = "America/Chicago"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *announcementsservicev1beta1.AnnouncementSubscription) error {
			if current.Status.Description != "OSOK recorded announcement subscription updated" || current.Status.PreferredTimeZone != "America/Chicago" {
				return fmt.Errorf("updated AnnouncementSubscription status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredAnnouncementSubscriptionEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
