/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocimock

import (
	"context"
	"fmt"
	"testing"

	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	ctrl "sigs.k8s.io/controller-runtime"
)

type lifecycleTestResource struct {
	Name    string
	Created bool
	Deleted bool
}

type lifecycleTestClient struct {
	createCalls int
	updateCalls int
	deleteCalls int
	retryDelete bool
}

func (c *lifecycleTestClient) CreateOrUpdate(_ context.Context, resource *lifecycleTestResource, _ ctrl.Request) (servicemanager.OSOKResponse, error) {
	if !resource.Created {
		c.createCalls++
		resource.Created = true
		return servicemanager.OSOKResponse{IsSuccessful: true}, nil
	}
	c.updateCalls++
	return servicemanager.OSOKResponse{IsSuccessful: true}, nil
}

func (c *lifecycleTestClient) Delete(_ context.Context, resource *lifecycleTestResource) (bool, error) {
	c.deleteCalls++
	if c.retryDelete && c.deleteCalls == 1 {
		return false, fmt.Errorf("retry delete")
	}
	resource.Deleted = true
	return true, nil
}

func TestRunLifecycleExecutesTypedCRUDContract(t *testing.T) {
	t.Parallel()

	resource := &lifecycleTestResource{Name: "created"}
	client := &lifecycleTestClient{}
	err := RunLifecycle(context.Background(), LifecycleScenario[*lifecycleTestResource]{
		Resource: resource,
		Client:   client,
		ValidateCreated: func(current *lifecycleTestResource) error {
			if !current.Created || current.Name != "created" {
				return fmt.Errorf("created resource = %+v", current)
			}
			return nil
		},
		Mutate: func(current *lifecycleTestResource) {
			current.Name = "updated"
		},
		ValidateUpdated: func(current *lifecycleTestResource) error {
			if current.Name != "updated" {
				return fmt.Errorf("updated resource = %+v", current)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.createCalls != 1 || client.updateCalls != 1 || client.deleteCalls != 1 || !resource.Deleted {
		t.Fatalf("calls create/update/delete=%d/%d/%d resource=%+v", client.createCalls, client.updateCalls, client.deleteCalls, resource)
	}
}

func TestRunLifecycleRejectsMutationWithoutUpdatedStateValidator(t *testing.T) {
	t.Parallel()

	resource := &lifecycleTestResource{Name: "created"}
	err := RunLifecycle(context.Background(), LifecycleScenario[*lifecycleTestResource]{
		Resource: resource,
		Client:   &lifecycleTestClient{},
		ValidateCreated: func(*lifecycleTestResource) error {
			return nil
		},
		Mutate: func(current *lifecycleTestResource) {
			current.Name = "updated"
		},
	})
	if err == nil {
		t.Fatal("RunLifecycle() error = nil, want missing update validator")
	}
}

func TestRunLifecycleRetriesClassifiedDeleteError(t *testing.T) {
	t.Parallel()

	resource := &lifecycleTestResource{Name: "created"}
	client := &lifecycleTestClient{retryDelete: true}
	err := RunLifecycle(context.Background(), LifecycleScenario[*lifecycleTestResource]{
		Resource: resource,
		Client:   client,
		ValidateCreated: func(*lifecycleTestResource) error {
			return nil
		},
		RetryDeleteError: func(err error) bool {
			return err != nil && err.Error() == "retry delete"
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.deleteCalls != 2 || !resource.Deleted {
		t.Fatalf("delete calls=%d resource=%+v", client.deleteCalls, resource)
	}
}
