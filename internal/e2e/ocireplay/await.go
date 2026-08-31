/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"context"
	"fmt"
	"time"
)

// Await repeatedly invokes a resource operation until it reports convergence.
// Replay mode advances immediately through recorded interactions; record mode
// respects the supplied OCI polling interval.
func Await(ctx context.Context, mode Mode, interval time.Duration, operation func() (bool, error)) error {
	if operation == nil {
		return fmt.Errorf("OCI replay await operation is required")
	}
	if mode != ModeRecord && mode != ModeReplay {
		return fmt.Errorf("OCI replay await mode %q is unsupported", mode)
	}
	if mode == ModeRecord && interval <= 0 {
		interval = 5 * time.Second
	}
	for attempt := 1; ; attempt++ {
		done, err := operation()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if mode == ModeReplay {
			if attempt >= 1000 {
				return fmt.Errorf("OCI replay await did not converge after %d attempts", attempt)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				continue
			}
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
