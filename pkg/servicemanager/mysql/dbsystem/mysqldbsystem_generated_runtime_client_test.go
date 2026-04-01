/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package dbsystem

import (
	"context"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	mysqlsdk "github.com/oracle/oci-go-sdk/v65/mysql"
	mysqlv1beta1 "github.com/oracle/oci-service-operator/api/mysql/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

type runtimeTestCredentialClient struct {
	getSecretFn    func(context.Context, string, string) (map[string][]byte, error)
	createSecretFn func(context.Context, string, string, map[string]string, map[string][]byte) (bool, error)
}

func (c *runtimeTestCredentialClient) CreateSecret(
	ctx context.Context,
	name string,
	namespace string,
	labels map[string]string,
	data map[string][]byte,
) (bool, error) {
	if c.createSecretFn != nil {
		return c.createSecretFn(ctx, name, namespace, labels, data)
	}
	return true, nil
}

func (c *runtimeTestCredentialClient) DeleteSecret(context.Context, string, string) (bool, error) {
	return true, nil
}

func (c *runtimeTestCredentialClient) GetSecret(ctx context.Context, name string, namespace string) (map[string][]byte, error) {
	if c.getSecretFn != nil {
		return c.getSecretFn(ctx, name, namespace)
	}
	return nil, nil
}

func (c *runtimeTestCredentialClient) UpdateSecret(context.Context, string, string, map[string]string, map[string][]byte) (bool, error) {
	return true, nil
}

type runtimeTestOCIClient struct {
	createFn func(context.Context, mysqlsdk.CreateDbSystemRequest) (mysqlsdk.CreateDbSystemResponse, error)
	getFn    func(context.Context, mysqlsdk.GetDbSystemRequest) (mysqlsdk.GetDbSystemResponse, error)
	listFn   func(context.Context, mysqlsdk.ListDbSystemsRequest) (mysqlsdk.ListDbSystemsResponse, error)
	updateFn func(context.Context, mysqlsdk.UpdateDbSystemRequest) (mysqlsdk.UpdateDbSystemResponse, error)
}

func (c *runtimeTestOCIClient) CreateDbSystem(ctx context.Context, request mysqlsdk.CreateDbSystemRequest) (mysqlsdk.CreateDbSystemResponse, error) {
	if c.createFn != nil {
		return c.createFn(ctx, request)
	}
	return mysqlsdk.CreateDbSystemResponse{}, nil
}

func (c *runtimeTestOCIClient) ListDbSystems(ctx context.Context, request mysqlsdk.ListDbSystemsRequest) (mysqlsdk.ListDbSystemsResponse, error) {
	if c.listFn != nil {
		return c.listFn(ctx, request)
	}
	return mysqlsdk.ListDbSystemsResponse{}, nil
}

func (c *runtimeTestOCIClient) GetDbSystem(ctx context.Context, request mysqlsdk.GetDbSystemRequest) (mysqlsdk.GetDbSystemResponse, error) {
	if c.getFn != nil {
		return c.getFn(ctx, request)
	}
	return mysqlsdk.GetDbSystemResponse{}, nil
}

func (c *runtimeTestOCIClient) UpdateDbSystem(ctx context.Context, request mysqlsdk.UpdateDbSystemRequest) (mysqlsdk.UpdateDbSystemResponse, error) {
	if c.updateFn != nil {
		return c.updateFn(ctx, request)
	}
	return mysqlsdk.UpdateDbSystemResponse{}, nil
}

func newRuntimeTestManager(t *testing.T, credClient *runtimeTestCredentialClient, ociClient MySQLDbSystemClientInterface) *MySqlDbSystemServiceManager {
	t.Helper()

	previousFactory := newMySqlDbSystemRuntimeOCIClient
	newMySqlDbSystemRuntimeOCIClient = func(common.ConfigurationProvider) (MySQLDbSystemClientInterface, error) {
		return ociClient, nil
	}
	t.Cleanup(func() {
		newMySqlDbSystemRuntimeOCIClient = previousFactory
	})

	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("test")}
	return NewMySqlDbSystemServiceManager(common.NewRawConfigurationProvider("", "", "", "", "", nil), credClient, nil, log, nil)
}

func runtimeActiveDbSystem(id string, displayName string) mysqlsdk.DbSystem {
	port := 3306
	portX := 33060
	description := "existing description"
	hostname := "mysql.example.com"
	ip := "10.0.0.1"
	availabilityDomain := "AD-1"
	faultDomain := "FAULT-DOMAIN-1"
	configurationID := "ocid1.mysqlconfiguration.oc1..cfg"
	return mysqlsdk.DbSystem{
		Id:                 common.String(id),
		DisplayName:        common.String(displayName),
		Description:        &description,
		LifecycleState:     mysqlsdk.DbSystemLifecycleStateActive,
		Port:               &port,
		PortX:              &portX,
		HostnameLabel:      &hostname,
		IpAddress:          &ip,
		AvailabilityDomain: &availabilityDomain,
		FaultDomain:        &faultDomain,
		ConfigurationId:    &configurationID,
		CompartmentId:      common.String("ocid1.compartment.oc1..example"),
	}
}

func TestGeneratedRuntimeMySqlDbSystemCreateUsesSecretInputsAndWritesEndpointSecret(t *testing.T) {
	const dbSystemID = "ocid1.mysqldbsystem.oc1..create"

	var createRequest mysqlsdk.CreateDbSystemRequest
	secretCreated := false
	credClient := &runtimeTestCredentialClient{
		getSecretFn: func(_ context.Context, name string, namespace string) (map[string][]byte, error) {
			if namespace != "default" {
				t.Fatalf("secret lookup namespace = %q, want default", namespace)
			}
			switch name {
			case "admin-user":
				return map[string][]byte{"username": []byte("admin")}, nil
			case "admin-password":
				return map[string][]byte{"password": []byte("Password123!")}, nil
			default:
				t.Fatalf("unexpected secret lookup %q", name)
				return nil, nil
			}
		},
		createSecretFn: func(_ context.Context, name string, namespace string, _ map[string]string, data map[string][]byte) (bool, error) {
			secretCreated = true
			if name != "test-dbsystem" || namespace != "default" {
				t.Fatalf("secret create target = %s/%s, want default/test-dbsystem", namespace, name)
			}
			if string(data["PrivateIPAddress"]) != "10.0.0.1" {
				t.Fatalf("PrivateIPAddress = %q, want 10.0.0.1", data["PrivateIPAddress"])
			}
			return true, nil
		},
	}

	manager := newRuntimeTestManager(t, credClient, &runtimeTestOCIClient{
		listFn: func(_ context.Context, request mysqlsdk.ListDbSystemsRequest) (mysqlsdk.ListDbSystemsResponse, error) {
			if request.DisplayName == nil || *request.DisplayName != "test-dbsystem" {
				t.Fatalf("list displayName = %v, want test-dbsystem", request.DisplayName)
			}
			return mysqlsdk.ListDbSystemsResponse{}, nil
		},
		createFn: func(_ context.Context, request mysqlsdk.CreateDbSystemRequest) (mysqlsdk.CreateDbSystemResponse, error) {
			createRequest = request
			return mysqlsdk.CreateDbSystemResponse{
				DbSystem: mysqlsdk.DbSystem{Id: common.String(dbSystemID)},
			}, nil
		},
		getFn: func(_ context.Context, request mysqlsdk.GetDbSystemRequest) (mysqlsdk.GetDbSystemResponse, error) {
			if request.DbSystemId == nil || *request.DbSystemId != dbSystemID {
				t.Fatalf("get dbSystemId = %v, want %s", request.DbSystemId, dbSystemID)
			}
			return mysqlsdk.GetDbSystemResponse{DbSystem: runtimeActiveDbSystem(dbSystemID, "test-dbsystem")}, nil
		},
	})

	resource := &mysqlv1beta1.MySqlDbSystem{}
	resource.Name = "test-dbsystem"
	resource.Namespace = "default"
	resource.Spec = mysqlv1beta1.MySqlDbSystemSpec{
		CompartmentId:        "ocid1.compartment.oc1..example",
		ShapeName:            "MySQL.VM.Standard.E4.1.8GB",
		AvailabilityDomain:   "AD-1",
		FaultDomain:          "FAULT-DOMAIN-1",
		DataStorageSizeInGBs: 50,
		SubnetId:             "ocid1.subnet.oc1..example",
		DisplayName:          "test-dbsystem",
		AdminUsername:        shared.UsernameSource{Secret: shared.SecretSource{SecretName: "admin-user"}},
		AdminPassword:        shared.PasswordSource{Secret: shared.SecretSource{SecretName: "admin-password"}},
	}

	response, err := manager.CreateOrUpdate(
		context.Background(),
		resource,
		ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default"}},
	)
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() should report success")
	}
	if createRequest.AdminUsername == nil || *createRequest.AdminUsername != "admin" {
		t.Fatalf("create adminUsername = %v, want admin", createRequest.AdminUsername)
	}
	if createRequest.AdminPassword == nil || *createRequest.AdminPassword != "Password123!" {
		t.Fatalf("create adminPassword = %v, want Password123!", createRequest.AdminPassword)
	}
	if string(resource.Status.OsokStatus.Ocid) != dbSystemID {
		t.Fatalf("status.ocid = %q, want %s", resource.Status.OsokStatus.Ocid, dbSystemID)
	}
	if !secretCreated {
		t.Fatal("endpoint secret was not created after ACTIVE response")
	}
}

func TestGeneratedRuntimeMySqlDbSystemCreateFallsBackToResourceNamespace(t *testing.T) {
	const dbSystemID = "ocid1.mysqldbsystem.oc1..fallback"

	secretLookups := make([]string, 0, 2)
	secretWrites := make([]string, 0, 1)
	credClient := &runtimeTestCredentialClient{
		getSecretFn: func(_ context.Context, name string, namespace string) (map[string][]byte, error) {
			secretLookups = append(secretLookups, namespace)
			switch name {
			case "admin-user":
				return map[string][]byte{"username": []byte("admin")}, nil
			case "admin-password":
				return map[string][]byte{"password": []byte("Password123!")}, nil
			default:
				t.Fatalf("unexpected secret lookup %q", name)
				return nil, nil
			}
		},
		createSecretFn: func(_ context.Context, _ string, namespace string, _ map[string]string, _ map[string][]byte) (bool, error) {
			secretWrites = append(secretWrites, namespace)
			return true, nil
		},
	}

	manager := newRuntimeTestManager(t, credClient, &runtimeTestOCIClient{
		listFn: func(_ context.Context, _ mysqlsdk.ListDbSystemsRequest) (mysqlsdk.ListDbSystemsResponse, error) {
			return mysqlsdk.ListDbSystemsResponse{}, nil
		},
		createFn: func(_ context.Context, _ mysqlsdk.CreateDbSystemRequest) (mysqlsdk.CreateDbSystemResponse, error) {
			return mysqlsdk.CreateDbSystemResponse{
				DbSystem: mysqlsdk.DbSystem{Id: common.String(dbSystemID)},
			}, nil
		},
		getFn: func(_ context.Context, _ mysqlsdk.GetDbSystemRequest) (mysqlsdk.GetDbSystemResponse, error) {
			return mysqlsdk.GetDbSystemResponse{DbSystem: runtimeActiveDbSystem(dbSystemID, "fallback-dbsystem")}, nil
		},
	})

	resource := &mysqlv1beta1.MySqlDbSystem{}
	resource.Name = "fallback-dbsystem"
	resource.Namespace = "resource-namespace"
	resource.Spec = mysqlv1beta1.MySqlDbSystemSpec{
		CompartmentId:        "ocid1.compartment.oc1..example",
		ShapeName:            "MySQL.VM.Standard.E4.1.8GB",
		AvailabilityDomain:   "AD-1",
		FaultDomain:          "FAULT-DOMAIN-1",
		DataStorageSizeInGBs: 50,
		SubnetId:             "ocid1.subnet.oc1..example",
		DisplayName:          "fallback-dbsystem",
		AdminUsername:        shared.UsernameSource{Secret: shared.SecretSource{SecretName: "admin-user"}},
		AdminPassword:        shared.PasswordSource{Secret: shared.SecretSource{SecretName: "admin-password"}},
	}

	response, err := manager.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() should report success")
	}
	if len(secretLookups) != 2 {
		t.Fatalf("secret lookups = %d, want 2", len(secretLookups))
	}
	for _, namespace := range secretLookups {
		if namespace != "resource-namespace" {
			t.Fatalf("secret lookup namespace = %q, want resource-namespace", namespace)
		}
	}
	if len(secretWrites) != 1 || secretWrites[0] != "resource-namespace" {
		t.Fatalf("secret writes = %v, want [resource-namespace]", secretWrites)
	}
}

func TestGeneratedRuntimeMySqlDbSystemBindExistingSkipsNoOpUpdates(t *testing.T) {
	const dbSystemID = "ocid1.mysqldbsystem.oc1..bind"

	updateCalled := false
	manager := newRuntimeTestManager(t, &runtimeTestCredentialClient{
		createSecretFn: func(context.Context, string, string, map[string]string, map[string][]byte) (bool, error) {
			return true, nil
		},
	}, &runtimeTestOCIClient{
		listFn: func(_ context.Context, _ mysqlsdk.ListDbSystemsRequest) (mysqlsdk.ListDbSystemsResponse, error) {
			return mysqlsdk.ListDbSystemsResponse{
				Items: []mysqlsdk.DbSystemSummary{
					{
						Id:             common.String(dbSystemID),
						LifecycleState: mysqlsdk.DbSystemLifecycleStateActive,
					},
				},
			}, nil
		},
		getFn: func(_ context.Context, request mysqlsdk.GetDbSystemRequest) (mysqlsdk.GetDbSystemResponse, error) {
			if request.DbSystemId == nil || *request.DbSystemId != dbSystemID {
				t.Fatalf("get dbSystemId = %v, want %s", request.DbSystemId, dbSystemID)
			}
			return mysqlsdk.GetDbSystemResponse{DbSystem: runtimeActiveDbSystem(dbSystemID, "bind-dbsystem")}, nil
		},
		updateFn: func(_ context.Context, _ mysqlsdk.UpdateDbSystemRequest) (mysqlsdk.UpdateDbSystemResponse, error) {
			updateCalled = true
			return mysqlsdk.UpdateDbSystemResponse{}, nil
		},
	})

	resource := &mysqlv1beta1.MySqlDbSystem{}
	resource.Name = "bind-dbsystem"
	resource.Namespace = "default"
	resource.Spec.DisplayName = "bind-dbsystem"
	resource.Spec.CompartmentId = "ocid1.compartment.oc1..example"

	response, err := manager.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() should report success")
	}
	if updateCalled {
		t.Fatal("UpdateDbSystem() should not be called when no mutable fields changed")
	}
	if string(resource.Status.OsokStatus.Ocid) != dbSystemID {
		t.Fatalf("status.ocid = %q, want %s", resource.Status.OsokStatus.Ocid, dbSystemID)
	}
}

func TestGeneratedRuntimeMySqlDbSystemBindLookupSkipsUnsupportedLifecycleStates(t *testing.T) {
	const dbSystemID = "ocid1.mysqldbsystem.oc1..active"

	manager := newRuntimeTestManager(t, &runtimeTestCredentialClient{
		createSecretFn: func(context.Context, string, string, map[string]string, map[string][]byte) (bool, error) {
			return true, nil
		},
	}, &runtimeTestOCIClient{
		listFn: func(_ context.Context, _ mysqlsdk.ListDbSystemsRequest) (mysqlsdk.ListDbSystemsResponse, error) {
			return mysqlsdk.ListDbSystemsResponse{
				Items: []mysqlsdk.DbSystemSummary{
					{
						Id:             common.String("ocid1.mysqldbsystem.oc1..deleting"),
						LifecycleState: mysqlsdk.DbSystemLifecycleStateDeleting,
					},
					{
						Id:             common.String(dbSystemID),
						LifecycleState: mysqlsdk.DbSystemLifecycleStateActive,
					},
				},
			}, nil
		},
		getFn: func(_ context.Context, request mysqlsdk.GetDbSystemRequest) (mysqlsdk.GetDbSystemResponse, error) {
			if request.DbSystemId == nil || *request.DbSystemId != dbSystemID {
				t.Fatalf("get dbSystemId = %v, want %s", request.DbSystemId, dbSystemID)
			}
			return mysqlsdk.GetDbSystemResponse{DbSystem: runtimeActiveDbSystem(dbSystemID, "bind-dbsystem")}, nil
		},
	})

	resource := &mysqlv1beta1.MySqlDbSystem{}
	resource.Name = "bind-dbsystem"
	resource.Namespace = "default"
	resource.Spec.DisplayName = "bind-dbsystem"
	resource.Spec.CompartmentId = "ocid1.compartment.oc1..example"

	response, err := manager.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() should report success")
	}
	if string(resource.Status.OsokStatus.Ocid) != dbSystemID {
		t.Fatalf("status.ocid = %q, want %s", resource.Status.OsokStatus.Ocid, dbSystemID)
	}
}

func TestGeneratedRuntimeMySqlDbSystemUpdateKeepsLegacyMutableSurface(t *testing.T) {
	const dbSystemID = "ocid1.mysqldbsystem.oc1..update"

	var updateRequest mysqlsdk.UpdateDbSystemRequest
	getCalls := 0
	manager := newRuntimeTestManager(t, &runtimeTestCredentialClient{
		createSecretFn: func(context.Context, string, string, map[string]string, map[string][]byte) (bool, error) {
			return true, nil
		},
	}, &runtimeTestOCIClient{
		getFn: func(_ context.Context, request mysqlsdk.GetDbSystemRequest) (mysqlsdk.GetDbSystemResponse, error) {
			getCalls++
			if request.DbSystemId == nil || *request.DbSystemId != dbSystemID {
				t.Fatalf("get dbSystemId = %v, want %s", request.DbSystemId, dbSystemID)
			}
			current := runtimeActiveDbSystem(dbSystemID, "old-name")
			return mysqlsdk.GetDbSystemResponse{DbSystem: current}, nil
		},
		updateFn: func(_ context.Context, request mysqlsdk.UpdateDbSystemRequest) (mysqlsdk.UpdateDbSystemResponse, error) {
			updateRequest = request
			return mysqlsdk.UpdateDbSystemResponse{}, nil
		},
	})

	resource := &mysqlv1beta1.MySqlDbSystem{}
	resource.Name = "update-dbsystem"
	resource.Namespace = "default"
	resource.Spec.MySqlDbSystemId = shared.OCID(dbSystemID)
	resource.Spec.DisplayName = "new-name"
	resource.Spec.HostnameLabel = "ignored-hostname"
	resource.Spec.MysqlVersion = "8.4.0"
	resource.Spec.Port = 3307

	response, err := manager.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() should report success")
	}
	if getCalls < 2 {
		t.Fatalf("GetDbSystem() calls = %d, want at least 2 for diff + follow-up", getCalls)
	}
	if updateRequest.DbSystemId == nil || *updateRequest.DbSystemId != dbSystemID {
		t.Fatalf("update dbSystemId = %v, want %s", updateRequest.DbSystemId, dbSystemID)
	}
	if updateRequest.UpdateDbSystemDetails.DisplayName == nil || *updateRequest.UpdateDbSystemDetails.DisplayName != "new-name" {
		t.Fatalf("update displayName = %v, want new-name", updateRequest.UpdateDbSystemDetails.DisplayName)
	}
	if updateRequest.UpdateDbSystemDetails.HostnameLabel != nil {
		t.Fatalf("update hostnameLabel = %v, want nil", updateRequest.UpdateDbSystemDetails.HostnameLabel)
	}
	if updateRequest.UpdateDbSystemDetails.MysqlVersion != nil {
		t.Fatalf("update mysqlVersion = %v, want nil", updateRequest.UpdateDbSystemDetails.MysqlVersion)
	}
	if updateRequest.UpdateDbSystemDetails.Port != nil {
		t.Fatalf("update port = %v, want nil", updateRequest.UpdateDbSystemDetails.Port)
	}
}

func TestGeneratedRuntimeMySqlDbSystemDeleteMatchesLegacyUnsupportedBehavior(t *testing.T) {
	manager := newRuntimeTestManager(t, &runtimeTestCredentialClient{}, &runtimeTestOCIClient{})

	done, err := manager.Delete(context.Background(), &mysqlv1beta1.MySqlDbSystem{})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !done {
		t.Fatal("Delete() should report success without issuing an OCI delete")
	}
}
