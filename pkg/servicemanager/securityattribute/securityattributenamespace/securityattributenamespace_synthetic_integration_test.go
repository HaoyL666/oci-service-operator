/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package securityattributenamespace

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	securityattributev1beta1 "github.com/oracle/oci-service-operator/api/securityattribute/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const syntheticSecurityAttributeNamespaceName = "osok_replay_security_namespace_v1"

func TestSyntheticSecurityAttributeNamespaceCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{Service: "securityattribute", Resource: "SecurityAttributeNamespace", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	sdkClient, closeSession := ocireplay.OpenSecurityAttributeSDK(t, ocireplay.ModeReplay, filepath.Join("testdata", "recordings", "securityattributenamespace_synthetic_crud.yaml"), metadata)
	resource := &securityattributev1beta1.SecurityAttributeNamespace{Spec: securityattributev1beta1.SecurityAttributeNamespaceSpec{
		CompartmentId: "ocid1.tenancy.oc1..replay", Name: syntheticSecurityAttributeNamespaceName, Description: "synthetic create", FreeformTags: map[string]string{"osok-replay": "synthetic"},
	}}
	client := newSecurityAttributeNamespaceServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*securityattributev1beta1.SecurityAttributeNamespace]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 0, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *securityattributev1beta1.SecurityAttributeNamespace) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *securityattributev1beta1.SecurityAttributeNamespace) error {
			if current.Status.Name != syntheticSecurityAttributeNamespaceName {
				return fmt.Errorf("created namespace status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *securityattributev1beta1.SecurityAttributeNamespace) {
			current.Spec.Description = "synthetic update"
		},
		ValidateUpdated: func(current *securityattributev1beta1.SecurityAttributeNamespace) error {
			if current.Status.Description != "synthetic update" {
				return fmt.Errorf("updated namespace status = %+v", current.Status)
			}
			return nil
		},
	})
}
