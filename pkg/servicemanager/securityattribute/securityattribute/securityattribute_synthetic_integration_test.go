/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package securityattribute

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	securityattributev1beta1 "github.com/oracle/oci-service-operator/api/securityattribute/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

const syntheticSecurityAttributeName = "osok_replay_security_attribute_v1"

func TestSyntheticSecurityAttributeCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{Service: "securityattribute", Resource: "SecurityAttribute", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	sdkClient, closeSession := ocireplay.OpenSecurityAttributeSDK(t, ocireplay.ModeReplay, filepath.Join("testdata", "recordings", "securityattribute_synthetic_crud.yaml"), metadata)
	resource := &securityattributev1beta1.SecurityAttribute{Spec: securityattributev1beta1.SecurityAttributeSpec{SecurityAttributeNamespaceId: "ocid1.securityattributenamespace.oc1..replay", Name: syntheticSecurityAttributeName, Description: "synthetic create"}}
	manager := &SecurityAttributeServiceManager{}
	hooks := newSecurityAttributeRuntimeHooks(manager, sdkClient)
	client := wrapSecurityAttributeGeneratedClient(hooks, defaultSecurityAttributeServiceClient{ServiceClient: generatedruntime.NewServiceClient[*securityattributev1beta1.SecurityAttribute](buildSecurityAttributeGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*securityattributev1beta1.SecurityAttribute]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 0, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *securityattributev1beta1.SecurityAttribute) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *securityattributev1beta1.SecurityAttribute) error {
			if current.Status.Name != syntheticSecurityAttributeName || current.Status.SecurityAttributeNamespaceId != resource.Spec.SecurityAttributeNamespaceId {
				return fmt.Errorf("created SecurityAttribute status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *securityattributev1beta1.SecurityAttribute) {
			current.Spec.Description = "synthetic update"
		},
		ValidateUpdated: func(current *securityattributev1beta1.SecurityAttribute) error {
			if current.Status.Description != "synthetic update" {
				return fmt.Errorf("updated SecurityAttribute status = %+v", current.Status)
			}
			return nil
		},
	})
}
