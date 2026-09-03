/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"context"
	"errors"
	"testing"

	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	ctrl "sigs.k8s.io/controller-runtime"
)

type lifecycleDeleteClient struct {
	deleted bool
	err     error
}

func (c lifecycleDeleteClient) CreateOrUpdate(context.Context, *struct{}, ctrl.Request) (servicemanager.OSOKResponse, error) {
	return servicemanager.OSOKResponse{IsSuccessful: true}, nil
}

func (c lifecycleDeleteClient) Delete(context.Context, *struct{}) (bool, error) {
	return c.deleted, c.err
}

func TestDeleteLifecycleResourceRetriesSelectedError(t *testing.T) {
	wantErr := errors.New("throttled")
	scenario := LifecycleScenario[*struct{}]{
		Resource: &struct{}{}, Client: lifecycleDeleteClient{err: wantErr},
		RetryDeleteError: func(err error) bool { return errors.Is(err, wantErr) },
	}
	deleted, err := deleteLifecycleResource(context.Background(), scenario)
	if err != nil || deleted {
		t.Fatalf("deleteLifecycleResource() = %t, %v; want retry", deleted, err)
	}
}

func TestDeleteLifecycleResourcePreservesUnselectedError(t *testing.T) {
	wantErr := errors.New("denied")
	scenario := LifecycleScenario[*struct{}]{
		Resource: &struct{}{}, Client: lifecycleDeleteClient{err: wantErr},
		RetryDeleteError: func(error) bool { return false },
	}
	deleted, err := deleteLifecycleResource(context.Background(), scenario)
	if deleted || !errors.Is(err, wantErr) {
		t.Fatalf("deleteLifecycleResource() = %t, %v; want %v", deleted, err, wantErr)
	}
}
