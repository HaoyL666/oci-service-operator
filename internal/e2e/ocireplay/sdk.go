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
	MaxBodyBytes int64
}

// SDKReplaySession owns one replay cassette and its configured OCI SDK base
// client. It never delegates to a network dispatcher.
type SDKReplaySession struct {
	cassette   *Cassette
	baseClient common.BaseClient
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
