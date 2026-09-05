/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package announcementsubscription

import (
	"path/filepath"
	"testing"

	announcementsservicesdk "github.com/oracle/oci-go-sdk/v65/announcementsservice"
	announcementsservicev1beta1 "github.com/oracle/oci-service-operator/api/announcementsservice/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationAnnouncementSubscriptionEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "announcementsubscription_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close AnnouncementSubscription OCI mock: %v", err)
		}
	})
	resource := &announcementsservicev1beta1.AnnouncementSubscription{}
	ocimock.InitializeResource(resource, "mock-announcementsubscription")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := announcementsservicesdk.AnnouncementSubscriptionClient{BaseClient: session.BaseClient()}
	manager := &AnnouncementSubscriptionServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}}
	hooks := newAnnouncementSubscriptionRuntimeHooks(manager, sdkClient)
	client := wrapAnnouncementSubscriptionGeneratedClient(hooks, defaultAnnouncementSubscriptionServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*announcementsservicev1beta1.AnnouncementSubscription](buildAnnouncementSubscriptionGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
