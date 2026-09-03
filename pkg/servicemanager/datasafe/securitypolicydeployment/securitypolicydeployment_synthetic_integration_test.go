/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package securitypolicydeployment

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	datasafesdk "github.com/oracle/oci-go-sdk/v65/datasafe"
	datasafev1beta1 "github.com/oracle/oci-service-operator/api/datasafe/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticSecurityPolicyDeploymentCreateReadDelete(t *testing.T) {
	resource := &datasafev1beta1.SecurityPolicyDeployment{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"osok-replay-securitypolicydeployment"},"spec":{"compartmentId":"ocid1.compartment.oc1..synthetic","targetId":"ocid1.targetdatabase.oc1..synthetic","targetType":"TARGET_DATABASE","securityPolicyId":"ocid1.securitypolicy.oc1..synthetic","displayName":"osok-replay-policy-deployment"}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.securitypolicydeployment.oc1..synthetic", "DEPLOYED", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "datasafe", Resource: "SecurityPolicyDeployment",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "securitypolicydeployment_synthetic_crud.yaml"), Host: "https://datasafe.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{
			CreatedBody: createdBody, EmptyCollectionBody: "{\"items\":[]}", PresentCollectionBody: "{\"items\":[" + createdBody + "]}", CreateStatus: 0, DeleteStatus: 0,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	manager := &SecurityPolicyDeploymentServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newSecurityPolicyDeploymentRuntimeHooks(manager, sdkClient)
	client := wrapSecurityPolicyDeploymentGeneratedClient(hooks, defaultSecurityPolicyDeploymentServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*datasafev1beta1.SecurityPolicyDeployment](buildSecurityPolicyDeploymentGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datasafev1beta1.SecurityPolicyDeployment]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *datasafev1beta1.SecurityPolicyDeployment) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *datasafev1beta1.SecurityPolicyDeployment) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created SecurityPolicyDeployment status has no OCI identity: %+v", current.Status)
			}
			return nil
		},
	})
}
