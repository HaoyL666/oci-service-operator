/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// SDKReplayOptions configures a credential-free OCI SDK replay session.
type SDKReplayOptions struct {
	Path         string
	Host         string
	BasePath     string
	Metadata     Metadata
	Bindings     map[string]string
	MaxBodyBytes int64
}

// SDKReplaySession owns one replay cassette and its configured OCI SDK base
// client. It never delegates to a network dispatcher.
type SDKReplaySession struct {
	cassette   *Cassette
	baseClient common.BaseClient
}

// SDKRecordOptions configures a live OCI SDK recording session. The supplied
// base client must already contain the intended configuration provider and
// signer.
type SDKRecordOptions struct {
	Path         string
	Metadata     Metadata
	BaseClient   *common.BaseClient
	Bindings     map[string]string
	Overwrite    bool
	MaxBodyBytes int64
}

// SDKRecordSession owns a recording cassette attached to a caller-provided OCI
// SDK base client.
type SDKRecordSession struct {
	cassette *Cassette
}

type unsignedReplaySigner struct{}

func (unsignedReplaySigner) Sign(*http.Request) error { return nil }

// OpenSDKReplay opens a checked-in cassette and returns a base client suitable
// for constructing the normal generated OCI SDK client.
func OpenSDKReplay(options SDKReplayOptions) (*SDKReplaySession, error) {
	host, err := replayHost(options.Host)
	if err != nil {
		return nil, err
	}
	if err := validateMetadata(options.Metadata); err != nil {
		return nil, err
	}
	cassette, err := Open(Options{
		Mode:         ModeReplay,
		Path:         options.Path,
		Metadata:     &options.Metadata,
		Bindings:     options.Bindings,
		MaxBodyBytes: options.MaxBodyBytes,
	})
	if err != nil {
		return nil, err
	}

	baseClient := common.DefaultBaseClientWithSigner(unsignedReplaySigner{})
	baseClient.Host = host
	baseClient.BasePath = strings.Trim(options.BasePath, "/")
	noRetry := common.NoRetryPolicy()
	baseClient.Configuration.RetryPolicy = &noRetry
	cassette.Attach(&baseClient)
	return &SDKReplaySession{cassette: cassette, baseClient: baseClient}, nil
}

// OpenSDKRecord attaches a sanitizer/recorder to an authenticated OCI SDK base
// client. Close atomically publishes the cassette after all cleanup traffic has
// completed.
func OpenSDKRecord(options SDKRecordOptions) (*SDKRecordSession, error) {
	if options.BaseClient == nil {
		return nil, fmt.Errorf("OCI recording base client is required")
	}
	if options.BaseClient.HTTPClient == nil {
		return nil, fmt.Errorf("OCI recording base client has no HTTP dispatcher")
	}
	if err := validateMetadata(options.Metadata); err != nil {
		return nil, err
	}
	cassette, err := Open(Options{
		Mode:         ModeRecord,
		Path:         options.Path,
		Metadata:     &options.Metadata,
		Bindings:     options.Bindings,
		Delegate:     options.BaseClient.HTTPClient,
		Overwrite:    options.Overwrite,
		MaxBodyBytes: options.MaxBodyBytes,
	})
	if err != nil {
		return nil, err
	}
	noRetry := common.NoRetryPolicy()
	options.BaseClient.Configuration.RetryPolicy = &noRetry
	cassette.Attach(options.BaseClient)
	return &SDKRecordSession{cassette: cassette}, nil
}

// BaseClient returns a copy of the configured OCI SDK base client.
func (s *SDKReplaySession) BaseClient() common.BaseClient {
	if s == nil {
		return common.BaseClient{}
	}
	return s.baseClient
}

// Metadata returns the cassette metadata.
func (s *SDKReplaySession) Metadata() *Metadata {
	if s == nil || s.cassette == nil {
		return nil
	}
	return s.cassette.Metadata()
}

// Close verifies that every replay interaction was consumed.
func (s *SDKReplaySession) Close() error {
	if s == nil || s.cassette == nil {
		return nil
	}
	return s.cassette.Close()
}

// Close atomically publishes the sanitized recording.
func (s *SDKRecordSession) Close() error {
	if s == nil || s.cassette == nil {
		return nil
	}
	return s.cassette.Close()
}

func replayHost(value string) (string, error) {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("OCI replay host %q must be an absolute HTTP(S) URL", value)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("OCI replay host %q must use HTTP or HTTPS", value)
	}
	if (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("OCI replay host %q must not contain a path, query, or fragment", value)
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}
