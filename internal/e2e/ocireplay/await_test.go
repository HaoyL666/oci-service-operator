/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAwaitReplayConvergesWithoutSleeping(t *testing.T) {
	t.Parallel()

	calls := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := Await(ctx, ModeReplay, time.Hour, func() (bool, error) {
		calls++
		return calls == 3, nil
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

func TestAwaitReturnsOperationAndContextErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("operation failed")
	if err := Await(context.Background(), ModeReplay, 0, func() (bool, error) { return false, want }); !errors.Is(err, want) {
		t.Fatalf("Await() error = %v, want %v", err, want)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Await(ctx, ModeRecord, time.Hour, func() (bool, error) { return false, nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("Await(cancelled) error = %v, want context cancellation", err)
	}
	if err := Await(context.Background(), ModeReplay, 0, func() (bool, error) { return false, nil }); err == nil || !strings.Contains(err.Error(), "1000 attempts") {
		t.Fatalf("Await(non-converging replay) error = %v", err)
	}
}
