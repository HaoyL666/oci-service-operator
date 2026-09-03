/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package transcriptionjob

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	aispeechsdk "github.com/oracle/oci-go-sdk/v65/aispeech"
	aispeechv1beta1 "github.com/oracle/oci-service-operator/api/aispeech/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticTranscriptionJobCreateReadDelete(t *testing.T) {
	resource := &aispeechv1beta1.TranscriptionJob{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"osok-replay-transcriptionjob"},"spec":{"compartmentId":"ocid1.compartment.oc1..synthetic","displayName":"osok-replay-transcription","inputLocation":{"locationType":"OBJECT_LIST_INLINE_INPUT_LOCATION","objectLocations":[{"namespaceName":"synthetic","bucketName":"input","objectNames":["audio.wav"]}]},"outputLocation":{"namespaceName":"synthetic","bucketName":"output","prefix":"transcripts"},"modelDetails":{"domain":"GENERIC","languageCode":"en-US"}}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.transcriptionjob.oc1..synthetic", "SUCCEEDED", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "aispeech", Resource: "TranscriptionJob",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "transcriptionjob_synthetic_crud.yaml"), Host: "https://speech.aiservice.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220101", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{
			CreatedBody: createdBody, EmptyCollectionBody: "{\"items\":[]}", PresentCollectionBody: "{\"items\":[" + createdBody + "]}", CreateStatus: 0, DeleteStatus: 0,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := aispeechsdk.AIServiceSpeechClient{BaseClient: session.BaseClient()}
	client := newTranscriptionJobServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*aispeechv1beta1.TranscriptionJob]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *aispeechv1beta1.TranscriptionJob) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *aispeechv1beta1.TranscriptionJob) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created TranscriptionJob status has no OCI identity: %+v", current.Status)
			}
			return nil
		},
	})
}
