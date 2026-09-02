/*
Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/
package kafkacluster

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	managedkafkasdk "github.com/oracle/oci-go-sdk/v65/managedkafka"
	managedkafkav1beta1 "github.com/oracle/oci-service-operator/api/managedkafka/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// A Kafka cluster allocates paid broker compute and storage.
func TestSyntheticKafkaClusterCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{Service: "managedkafka", Resource: "KafkaCluster", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: filepath.Join("testdata", "recordings", "kafkacluster_synthetic_crud.yaml"), Host: "https://kafka.us-ashburn-1.oci.oraclecloud.com", BasePath: "20240901", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := managedkafkasdk.KafkaClusterClient{BaseClient: session.BaseClient()}
	hooks := newKafkaClusterDefaultRuntimeHooks(sdkClient)
	applyKafkaClusterRuntimeHooks(&hooks, sdkClient, nil)
	client := wrapKafkaClusterGeneratedClient(hooks, defaultKafkaClusterServiceClient{ServiceClient: generatedruntime.NewServiceClient[*managedkafkav1beta1.KafkaCluster](buildKafkaClusterGeneratedRuntimeConfig(&KafkaClusterServiceManager{}, hooks))})
	resource := newTestKafkaCluster()
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*managedkafkav1beta1.KafkaCluster]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) }, HasIdentity: func(current *managedkafkav1beta1.KafkaCluster) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *managedkafkav1beta1.KafkaCluster) error {
		if current.Status.DisplayName != "kafka-cluster-sample" || current.Status.LifecycleState != "ACTIVE" {
			return fmt.Errorf("created KafkaCluster status = %+v", current.Status)
		}
		return nil
	}, Mutate: func(current *managedkafkav1beta1.KafkaCluster) { current.Spec.DisplayName = "kafka-cluster-updated" }, ValidateUpdated: func(current *managedkafkav1beta1.KafkaCluster) error {
		if current.Status.DisplayName != "kafka-cluster-updated" {
			return fmt.Errorf("updated KafkaCluster status = %+v", current.Status)
		}
		return nil
	}})
}
