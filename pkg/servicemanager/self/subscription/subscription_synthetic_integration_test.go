/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package subscription

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	selfsdk "github.com/oracle/oci-go-sdk/v65/self"
	selfv1beta1 "github.com/oracle/oci-service-operator/api/self/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticSubscriptionCreateReadDelete(t *testing.T) {
	resource := makeSubscriptionResource()
	createdBody, err := ocireplay.SyntheticJSONBody(makeSDKSubscription(testSubscriptionID, selfsdk.LifecycleStateEnumActive, selfsdk.LifecycleDetailsEnumActive))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(makeSubscriptionWorkRequest("ocid1.workrequest.oc1..syntheticcreate", selfsdk.OperationStatusSucceeded, selfsdk.OperationTypeCreateSubscription, selfsdk.ActionTypeCreated, testSubscriptionID))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(makeSubscriptionWorkRequest("ocid1.workrequest.oc1..syntheticdelete", selfsdk.OperationStatusSucceeded, selfsdk.OperationTypeDeleteSubscription, selfsdk.ActionTypeDeleted, testSubscriptionID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "self", Resource: "Subscription", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "subscription_synthetic_crud.yaml"), Host: "https://self.us-ashburn-1.oci.oraclecloud.com", BasePath: "20260129", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := selfsdk.SubscriptionClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	client := newSubscriptionServiceClientWithOCIClient(log, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*selfv1beta1.Subscription]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *selfv1beta1.Subscription) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *selfv1beta1.Subscription) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created Subscription status = %+v", current.Status)
		}
		return nil
	}})
}
