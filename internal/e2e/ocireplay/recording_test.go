/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import "testing"

func TestRequestedModeAndOverwrite(t *testing.T) {
	t.Setenv(CassetteModeEnv, "")
	mode, err := RequestedMode()
	if err != nil || mode != ModeReplay {
		t.Fatalf("RequestedMode() = %q, %v; want replay", mode, err)
	}
	t.Setenv(CassetteModeEnv, "record")
	mode, err = RequestedMode()
	if err != nil || mode != ModeRecord {
		t.Fatalf("RequestedMode() = %q, %v; want record", mode, err)
	}
	t.Setenv(CassetteModeEnv, "unknown")
	if _, err := RequestedMode(); err == nil {
		t.Fatal("RequestedMode() error = nil for unknown mode")
	}

	t.Setenv(CassetteOverwriteEnv, "yes")
	if !RecordingOverwriteRequested() {
		t.Fatal("RecordingOverwriteRequested() = false, want true")
	}
	t.Setenv(CassetteOverwriteEnv, "false")
	if RecordingOverwriteRequested() {
		t.Fatal("RecordingOverwriteRequested() = true, want false")
	}
}

func TestRecordingConfigurationProviderRejectsUnknownAuth(t *testing.T) {
	t.Setenv(RecordingAuthEnv, "unknown")
	if _, err := RecordingConfigurationProvider(); err == nil {
		t.Fatal("RecordingConfigurationProvider() error = nil for unknown auth")
	}
}
