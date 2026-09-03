/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package transcriptionjob

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	aispeechsdk "github.com/oracle/oci-go-sdk/v65/aispeech"
	aispeechv1beta1 "github.com/oracle/oci-service-operator/api/aispeech/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedTranscriptionJobName = "osok-replay-transcription-job-v3"

func TestRecordedTranscriptionJobCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	namespaceName := "replaynamespace"
	bucketName := "replay-speech-bucket"
	objectName := "audio.wav"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredTranscriptionJobRecordingEnv(t, "OCI_COMPARTMENT_ID")
		namespaceName = requiredTranscriptionJobRecordingEnv(t, "OCI_REPLAY_SPEECH_NAMESPACE")
		bucketName = requiredTranscriptionJobRecordingEnv(t, "OCI_REPLAY_SPEECH_BUCKET")
		objectName = requiredTranscriptionJobRecordingEnv(t, "OCI_REPLAY_SPEECH_OBJECT")
	}
	sdkClient, closeSession := openRecordedTranscriptionJobSDK(t, mode, namespaceName, bucketName)
	resource := &aispeechv1beta1.TranscriptionJob{
		ObjectMeta: metav1.ObjectMeta{Name: "recorded-transcription-job-v3"},
		Spec: aispeechv1beta1.TranscriptionJobSpec{
			CompartmentId: compartmentID,
			DisplayName:   recordedTranscriptionJobName,
			Description:   "recorded create",
			InputLocation: aispeechv1beta1.TranscriptionJobInputLocation{
				LocationType: "OBJECT_LIST_INLINE_INPUT_LOCATION",
				ObjectLocations: []aispeechv1beta1.TranscriptionJobInputLocationObjectLocation{{
					NamespaceName: namespaceName,
					BucketName:    bucketName,
					ObjectNames:   []string{objectName},
				}},
			},
			OutputLocation: aispeechv1beta1.TranscriptionJobOutputLocation{
				NamespaceName: namespaceName,
				BucketName:    bucketName,
				Prefix:        "transcripts/",
			},
			ModelDetails: aispeechv1beta1.TranscriptionJobModelDetails{
				Domain:       "GENERIC",
				LanguageCode: "en-US",
			},
			FreeformTags: map[string]string{"osok-replay": "create"},
		},
	}
	client := newTranscriptionJobServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*aispeechv1beta1.TranscriptionJob]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, Timeout: 20 * time.Minute, CleanupTimeout: 10 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		RetryError: func(err error) bool {
			return ocireplay.IsHTTPStatus(err, 404) || ocireplay.IsHTTPStatus(err, 409)
		},
		HasIdentity: func(current *aispeechv1beta1.TranscriptionJob) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *aispeechv1beta1.TranscriptionJob) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.LifecycleState != string(aispeechsdk.TranscriptionJobLifecycleStateSucceeded) {
				return fmt.Errorf("created TranscriptionJob status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *aispeechv1beta1.TranscriptionJob) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *aispeechv1beta1.TranscriptionJob) error {
			if current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated TranscriptionJob status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedTranscriptionJobSDK(t *testing.T, mode ocireplay.Mode, namespaceName string, bucketName string) (aispeechsdk.AIServiceSpeechClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "aispeech", Resource: "TranscriptionJob",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "transcriptionjob_crud.yaml")
	bindings := map[string]string{"speech-namespace": namespaceName, "speech-bucket": bucketName}
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := aispeechsdk.NewAIServiceSpeechClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path: path, Host: "https://speech.aiservice.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220101", Metadata: metadata, Bindings: bindings,
	})
	if err != nil {
		t.Fatal(err)
	}
	return aispeechsdk.AIServiceSpeechClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredTranscriptionJobRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
