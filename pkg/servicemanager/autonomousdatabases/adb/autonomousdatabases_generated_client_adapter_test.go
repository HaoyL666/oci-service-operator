/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package adb

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	databasev1beta1 "github.com/oracle/oci-service-operator/api/database/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
)

func discardAutonomousDatabasesLogger() loggerutil.OSOKLogger {
	return loggerutil.OSOKLogger{Logger: logr.Discard()}
}

func TestNewAutonomousDatabasesServiceManagerDefaultsToLegacyAdapter(t *testing.T) {
	t.Parallel()

	manager := NewAutonomousDatabasesServiceManagerWithDeps(servicemanager.RuntimeDeps{
		Log: discardAutonomousDatabasesLogger(),
	})

	client, ok := manager.client.(legacyAutonomousDatabasesServiceClient)
	if !ok {
		t.Fatalf("manager.client = %T, want legacyAutonomousDatabasesServiceClient", manager.client)
	}
	if client.delegate == nil {
		t.Fatal("legacy adapter delegate should be initialized")
	}
}

func TestLegacyAutonomousDatabasesServiceClientDeleteReturnsExplicitUnsupportedOutcome(t *testing.T) {
	t.Parallel()

	client := legacyAutonomousDatabasesServiceClient{
		delegate: NewAdbServiceManagerWithDeps(servicemanager.RuntimeDeps{
			Log: discardAutonomousDatabasesLogger(),
		}),
	}
	resource := &databasev1beta1.AutonomousDatabases{}

	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("Delete() should allow finalizer release for unsupported ADB deletes")
	}
	if !strings.Contains(resource.Status.OsokStatus.Message, "delete is not supported") {
		t.Fatalf("status.message = %q, want explicit unsupported delete message", resource.Status.OsokStatus.Message)
	}
	if resource.Status.OsokStatus.Reason != string(shared.Terminating) {
		t.Fatalf("status.reason = %q, want %q", resource.Status.OsokStatus.Reason, shared.Terminating)
	}
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatal("status.deletedAt should be recorded for unsupported delete completion")
	}
	if len(resource.Status.OsokStatus.Conditions) == 0 || resource.Status.OsokStatus.Conditions[len(resource.Status.OsokStatus.Conditions)-1].Type != shared.Terminating {
		t.Fatalf("status conditions = %#v, want trailing Terminating condition", resource.Status.OsokStatus.Conditions)
	}
}
