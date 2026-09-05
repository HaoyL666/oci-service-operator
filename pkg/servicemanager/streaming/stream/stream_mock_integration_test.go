/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package stream

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	streamingsdk "github.com/oracle/oci-go-sdk/v65/streaming"
	streamingv1beta1 "github.com/oracle/oci-service-operator/api/streaming/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const mockStreamID = "ocid1.stream.oc1..mock"

// Contract evidence:
//   - recorded OCI trace: testdata/recordings/stream_crud.yaml
//   - formal contract: formal/controllers/streaming/stream and formal/imports/streaming/stream.json
//   - endpoint Secret runtime: stream_endpoint_secret_client.go
//   - OCI SDK: vendor/github.com/oracle/oci-go-sdk/v65/streaming
func TestMockIntegrationStreamLifecycleCRUD(t *testing.T) {
	t.Parallel()

	resource := &streamingv1beta1.Stream{
		ObjectMeta: metav1.ObjectMeta{Name: "mock-stream", Namespace: "default", UID: types.UID("mock-stream-uid")},
		Spec: streamingv1beta1.StreamSpec{
			Name:             "mock-stream",
			Partitions:       1,
			CompartmentId:    "ocid1.compartment.oc1..mock",
			RetentionInHours: 24,
			FreeformTags:     map[string]string{"osok-mock": "create"},
		},
	}
	responder, err := newStreamMockResponder(resource)
	if err != nil {
		t.Fatal(err)
	}
	session, err := ocimock.Open(ocimock.Options{Host: "https://streaming.mock.invalid", BasePath: "20180418", Responder: responder})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Stream OCI mock: %v", err)
		}
	})

	client := newRecordedStreamClient(streamingsdk.StreamAdminClient{BaseClient: session.BaseClient()})
	err = ocimock.RunLifecycle(context.Background(), ocimock.LifecycleScenario[*streamingv1beta1.Stream]{
		Resource:      resource,
		Client:        client,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		ValidateCreated: func(current *streamingv1beta1.Stream) error {
			if current.Status.Id != mockStreamID ||
				current.Status.Name != "mock-stream" ||
				current.Status.MessagesEndpoint != "https://messages.mock.invalid" ||
				current.Status.LifecycleState != string(streamingsdk.StreamLifecycleStateActive) {
				return fmt.Errorf("created Stream status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *streamingv1beta1.Stream) {
			current.Spec.FreeformTags = map[string]string{"osok-mock": "update"}
		},
		ValidateUpdated: func(current *streamingv1beta1.Stream) error {
			if current.Status.FreeformTags["osok-mock"] != "update" ||
				current.Status.LifecycleState != string(streamingsdk.StreamLifecycleStateActive) {
				return fmt.Errorf("updated Stream status = %+v", current.Status)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func newStreamMockResponder(resource *streamingv1beta1.Stream) (*ocimock.CRUDResponder[streamingsdk.Stream], error) {
	createdAt := common.SDKTime{Time: time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)}
	createReadObserved := false
	updateReadObserved := false
	return ocimock.NewCRUDResponder(ocimock.CRUDOptions[streamingsdk.Stream]{
		CollectionPath:     "/20180418/streams",
		ItemPath:           "/20180418/streams/" + mockStreamID,
		ExpectedOperations: []ocimock.Operation{ocimock.OperationCreate, ocimock.OperationRead, ocimock.OperationUpdate, ocimock.OperationDelete},
		RequireCreateRead:  true,
		RequireUpdateRead:  true,
		Create: func(request ocimock.Request) (streamingsdk.Stream, ocimock.Response, error) {
			var details streamingsdk.CreateStreamDetails
			if err := ocimock.DecodeJSONRequest(request, &details); err != nil {
				return streamingsdk.Stream{}, ocimock.Response{}, err
			}
			if err := ocimock.ValidateMandatoryFields(details); err != nil {
				return streamingsdk.Stream{}, ocimock.Response{}, err
			}
			if details.Name == nil || *details.Name != resource.Spec.Name ||
				details.Partitions == nil || *details.Partitions != resource.Spec.Partitions ||
				details.CompartmentId == nil || *details.CompartmentId != resource.Spec.CompartmentId ||
				details.RetentionInHours == nil || *details.RetentionInHours != resource.Spec.RetentionInHours {
				return streamingsdk.Stream{}, ocimock.Response{}, fmt.Errorf("unexpected CreateStream details: %+v", details)
			}
			state := streamingsdk.Stream{
				Name:             details.Name,
				Id:               common.String(mockStreamID),
				Partitions:       details.Partitions,
				RetentionInHours: details.RetentionInHours,
				CompartmentId:    details.CompartmentId,
				StreamPoolId:     common.String("ocid1.streampool.oc1..mock"),
				LifecycleState:   streamingsdk.StreamLifecycleStateCreating,
				TimeCreated:      &createdAt,
				MessagesEndpoint: common.String("https://messages.mock.invalid"),
				FreeformTags:     details.FreeformTags,
			}
			response, err := ocimock.JSONResponse(http.StatusOK, state)
			return state, response, err
		},
		ReadTransition: func(_ ocimock.Request, state streamingsdk.Stream) (streamingsdk.Stream, ocimock.Response, error) {
			switch state.LifecycleState {
			case streamingsdk.StreamLifecycleStateCreating:
				if createReadObserved {
					state.LifecycleState = streamingsdk.StreamLifecycleStateActive
				} else {
					createReadObserved = true
				}
			case streamingsdk.StreamLifecycleStateUpdating:
				if updateReadObserved {
					state.LifecycleState = streamingsdk.StreamLifecycleStateActive
				} else {
					updateReadObserved = true
				}
			}
			response, err := ocimock.JSONResponse(http.StatusOK, state)
			return state, response, err
		},
		Update: func(request ocimock.Request, state streamingsdk.Stream) (streamingsdk.Stream, ocimock.Response, error) {
			var details streamingsdk.UpdateStreamDetails
			if err := ocimock.DecodeJSONRequest(request, &details); err != nil {
				return streamingsdk.Stream{}, ocimock.Response{}, err
			}
			if details.FreeformTags["osok-mock"] != "update" || details.StreamPoolId != nil {
				return streamingsdk.Stream{}, ocimock.Response{}, fmt.Errorf("unexpected UpdateStream details: %+v", details)
			}
			state.FreeformTags = details.FreeformTags
			state.LifecycleState = streamingsdk.StreamLifecycleStateUpdating
			response, err := ocimock.JSONResponse(http.StatusOK, state)
			return state, response, err
		},
		Delete: func(_ ocimock.Request, _ streamingsdk.Stream) (ocimock.Response, error) {
			return ocimock.EmptyResponse(http.StatusNoContent), nil
		},
	})
}
