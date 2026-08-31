/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
)

const (
	CassetteModeEnv      = "OSOK_OCI_CASSETTE_MODE"
	CassetteOverwriteEnv = "OSOK_OCI_CASSETTE_OVERWRITE"
	RecordingAuthEnv     = "OSOK_OCI_RECORD_AUTH"
)

// RequestedMode returns replay by default and accepts record only when the
// operator opts into live OCI traffic explicitly.
func RequestedMode() (Mode, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(CassetteModeEnv)))
	switch value {
	case "", string(ModeReplay):
		return ModeReplay, nil
	case string(ModeRecord):
		return ModeRecord, nil
	default:
		return "", fmt.Errorf("%s=%q is unsupported; expected replay or record", CassetteModeEnv, value)
	}
}

// RecordingOverwriteRequested reports whether a live recording may replace an
// existing cassette.
func RecordingOverwriteRequested() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(CassetteOverwriteEnv))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

// RecordingConfigurationProvider loads an explicitly selected OCI config
// profile for a live recording. It supports user-principal and security-token
// profiles without exposing their contents to the cassette.
func RecordingConfigurationProvider() (common.ConfigurationProvider, error) {
	configPath := strings.TrimSpace(os.Getenv("OCI_CONFIG_FILE"))
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve OCI config home: %w", err)
		}
		configPath = filepath.Join(home, ".oci", "config")
	}
	profile := strings.TrimSpace(os.Getenv("OCI_CONFIG_PROFILE"))
	if profile == "" {
		profile = "DEFAULT"
	}
	passphrase := os.Getenv("OCI_CONFIG_PASSPHRASE")
	authMode := strings.ToLower(strings.TrimSpace(os.Getenv(RecordingAuthEnv)))
	if authMode == "" {
		authMode = strings.ToLower(strings.TrimSpace(os.Getenv("OCI_CLI_AUTH")))
	}
	switch authMode {
	case "", "user_principal":
		provider, err := common.ConfigurationProviderFromFileWithProfile(configPath, profile, passphrase)
		if err != nil {
			return nil, fmt.Errorf("create user-principal recording provider: %w", err)
		}
		return provider, nil
	case "security_token":
		provider, err := common.ConfigurationProviderForSessionTokenWithProfile(configPath, profile, passphrase)
		if err != nil {
			return nil, fmt.Errorf("create security-token recording provider: %w", err)
		}
		return provider, nil
	default:
		return nil, fmt.Errorf("%s=%q is unsupported; expected user_principal or security_token", RecordingAuthEnv, authMode)
	}
}
