/*
Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/
package rediscluster

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	redissdk "github.com/oracle/oci-go-sdk/v65/redis"
	redisv1beta1 "github.com/oracle/oci-service-operator/api/redis/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// A Redis cluster allocates paid cache nodes and private-network capacity.
func TestSyntheticRedisClusterCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{Service: "redis", Resource: "RedisCluster", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: filepath.Join("testdata", "recordings", "rediscluster_synthetic_crud.yaml"), Host: "https://redis.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220315", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := redissdk.RedisClusterClient{BaseClient: session.BaseClient()}
	client := newRedisTestManager(sdkClient).client
	resource := makeSpecRedisCluster()
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*redisv1beta1.RedisCluster]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) }, HasIdentity: func(current *redisv1beta1.RedisCluster) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *redisv1beta1.RedisCluster) error {
		if current.Status.DisplayName != "redis-sample" || current.Status.LifecycleState != "ACTIVE" {
			return fmt.Errorf("created RedisCluster status = %+v", current.Status)
		}
		return nil
	}, Mutate: func(current *redisv1beta1.RedisCluster) { current.Spec.DisplayName = "redis-updated" }, ValidateUpdated: func(current *redisv1beta1.RedisCluster) error {
		if current.Status.DisplayName != "redis-updated" {
			return fmt.Errorf("updated RedisCluster status = %+v", current.Status)
		}
		return nil
	}})
}
