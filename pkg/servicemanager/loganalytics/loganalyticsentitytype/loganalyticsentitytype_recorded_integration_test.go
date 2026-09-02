/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package loganalyticsentitytype

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	objectstoragesdk "github.com/oracle/oci-go-sdk/v65/objectstorage"
	loganalyticsv1beta1 "github.com/oracle/oci-service-operator/api/loganalytics/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type recordedEntityTypeNamespaceGetter struct{ namespace string }

func (g recordedEntityTypeNamespaceGetter) GetNamespace(context.Context, objectstoragesdk.GetNamespaceRequest) (objectstoragesdk.GetNamespaceResponse, error) {
	return objectstoragesdk.GetNamespaceResponse{Value: &g.namespace}, nil
}

func TestRecordedLogAnalyticsEntityTypeCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "loganalytics", Resource: "LogAnalyticsEntityType", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	namespaceName, entityTypeName := "replay-namespace", "OSOKReplayEntityType"
	if mode == ocireplay.ModeRecord {
		namespaceName = requiredEntityTypeRecordingEnv(t, "OCI_REPLAY_LOG_ANALYTICS_NAMESPACE")
		entityTypeName = fmt.Sprintf("OSOKReplayEntityType%d", time.Now().Unix())
	}
	sdkClient, closeSession := ocireplay.OpenLogAnalyticsSDK(t, mode, filepath.Join("testdata", "recordings", "loganalyticsentitytype_crud.yaml"), metadata, map[string]string{
		"loganalytics-namespace": namespaceName,
		"entity-type-name":       entityTypeName,
	})
	resource := &loganalyticsv1beta1.LogAnalyticsEntityType{
		ObjectMeta: metav1.ObjectMeta{Name: "osok-replay-entity-type", Namespace: namespaceName},
		Spec: loganalyticsv1beta1.LogAnalyticsEntityTypeSpec{
			Name:       entityTypeName,
			Category:   "osok-replay",
			Properties: []loganalyticsv1beta1.LogAnalyticsEntityTypeProperty{{Name: "host", Description: "recorded hostname"}},
		},
	}
	client := newLogAnalyticsEntityTypeServiceClientWithOCIClientAndNamespaceGetter(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
		recordedEntityTypeNamespaceGetter{namespace: namespaceName},
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*loganalyticsv1beta1.LogAnalyticsEntityType]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *loganalyticsv1beta1.LogAnalyticsEntityType) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *loganalyticsv1beta1.LogAnalyticsEntityType) error {
			if current.Status.Name != entityTypeName || current.Status.InternalName == "" {
				return fmt.Errorf("created LogAnalyticsEntityType status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *loganalyticsv1beta1.LogAnalyticsEntityType) {
			current.Spec.Category = "osok-replay-updated"
		},
		ValidateUpdated: func(current *loganalyticsv1beta1.LogAnalyticsEntityType) error {
			if current.Status.Category != "osok-replay-updated" {
				return fmt.Errorf("updated LogAnalyticsEntityType status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredEntityTypeRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
