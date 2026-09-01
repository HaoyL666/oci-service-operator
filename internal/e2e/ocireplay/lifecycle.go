/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	ctrl "sigs.k8s.io/controller-runtime"
)

// LifecycleClient is the common service-manager seam exercised by OCI SDK
// record/replay lifecycle scenarios.
type LifecycleClient[T any] interface {
	CreateOrUpdate(context.Context, T, ctrl.Request) (servicemanager.OSOKResponse, error)
	Delete(context.Context, T) (bool, error)
}

// LifecycleScenario keeps repetitive record/replay orchestration in one place
// while leaving resource construction, mutation, and assertions typed in the
// owning service-manager package.
type LifecycleScenario[T any] struct {
	Mode            Mode
	Resource        T
	Client          LifecycleClient[T]
	CloseSession    func() error
	PollInterval    time.Duration
	Timeout         time.Duration
	CleanupTimeout  time.Duration
	CreateContext   func(context.Context) context.Context
	HasIdentity     func(T) bool
	ValidateCreated func(T) error
	Mutate          func(T)
	ValidateUpdated func(T) error
	RetryError      func(error) bool
}

// RunLifecycle records or replays create, read, update, and confirmed delete.
// Record-mode cleanup runs on every failure after an OCI identity is known.
func RunLifecycle[T any](t *testing.T, scenario LifecycleScenario[T]) {
	t.Helper()
	if scenario.CloseSession == nil {
		t.Fatal("lifecycle cassette close function is required")
	}
	if scenario.Timeout <= 0 {
		scenario.Timeout = 20 * time.Minute
	}
	if scenario.CleanupTimeout <= 0 {
		scenario.CleanupTimeout = 5 * time.Minute
	}

	closed := false
	t.Cleanup(func() {
		if !closed && scenario.Mode == ModeReplay {
			if err := scenario.CloseSession(); err != nil {
				t.Errorf("close lifecycle cassette: %v", err)
			}
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), scenario.Timeout)
	defer cancel()
	deleted := false
	if scenario.Mode == ModeRecord {
		t.Cleanup(func() {
			if deleted || (scenario.HasIdentity != nil && !scenario.HasIdentity(scenario.Resource)) {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), scenario.CleanupTimeout)
			defer cleanupCancel()
			_ = Await(cleanupCtx, scenario.Mode, scenario.PollInterval, func() (bool, error) {
				return scenario.Client.Delete(cleanupCtx, scenario.Resource)
			})
		})
	}

	createCtx := ctx
	if scenario.CreateContext != nil {
		createCtx = scenario.CreateContext(ctx)
	}
	if err := awaitLifecycleConvergence(createCtx, scenario); err != nil {
		t.Fatal(err)
	}
	if scenario.ValidateCreated != nil {
		if err := scenario.ValidateCreated(scenario.Resource); err != nil {
			t.Fatal(err)
		}
	}
	if scenario.Mutate != nil {
		scenario.Mutate(scenario.Resource)
		if err := awaitLifecycleConvergence(ctx, scenario); err != nil {
			t.Fatal(err)
		}
	}
	if scenario.ValidateUpdated != nil {
		if err := scenario.ValidateUpdated(scenario.Resource); err != nil {
			t.Fatal(err)
		}
	}
	if err := Await(ctx, scenario.Mode, scenario.PollInterval, func() (bool, error) {
		return scenario.Client.Delete(ctx, scenario.Resource)
	}); err != nil {
		t.Fatal(err)
	}
	deleted = true
	if err := scenario.CloseSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func awaitLifecycleConvergence[T any](ctx context.Context, scenario LifecycleScenario[T]) error {
	return Await(ctx, scenario.Mode, scenario.PollInterval, func() (bool, error) {
		response, err := scenario.Client.CreateOrUpdate(ctx, scenario.Resource, ctrl.Request{})
		if err != nil {
			if scenario.RetryError != nil && scenario.RetryError(err) {
				return false, nil
			}
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("lifecycle reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}
