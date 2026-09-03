/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/common"
)

const syntheticRecordEnv = "OSOK_OCI_SYNTHETIC_RECORD"

// SyntheticResponse describes one contract-authored OCI response used only
// while deterministically refreshing a synthetic cassette.
type SyntheticResponse struct {
	StatusCode int
	Headers    http.Header
	Body       string
}

// SyntheticResponder returns a contract-authored response for an SDK request
// while a synthetic cassette is being explicitly refreshed.
type SyntheticResponder func(*http.Request) (SyntheticResponse, error)

// SyntheticCRUDOptions describes the normal synchronous CRUD surface used by
// generated service managers. The responder exists only for cassette
// authoring; checked-in tests replay the resulting requests strictly.
type SyntheticCRUDOptions struct {
	CollectionPath        string
	CreatedBody           string
	UpdatedBody           string
	EmptyCollectionBody   string
	PresentCollectionBody string
	CreateStatus          int
	UpdateStatus          int
	DeleteStatus          int
	NotFoundCode          string
}

// SDKSyntheticOptions configures a strict replay session by default. Setting
// OSOK_OCI_SYNTHETIC_RECORD=true records the real OCI SDK requests against
// Responses instead, and still requires OCI_REPLAY_OVERWRITE=true before an
// existing cassette can be replaced.
type SDKSyntheticOptions struct {
	Path         string
	Host         string
	BasePath     string
	Metadata     Metadata
	Bindings     map[string]string
	Responses    []SyntheticResponse
	Responder    SyntheticResponder
	MaxBodyBytes int64
}

// SyntheticObservedBody projects a CR spec into a JSON response fixture and
// adds the common OCI identity and lifecycle fields. Extra fields may refine
// resource-specific response contracts.
func SyntheticObservedBody(spec any, id string, lifecycleState string, extra map[string]any) (string, error) {
	encoded, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("marshal synthetic spec: %w", err)
	}
	fields := map[string]any{}
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return "", fmt.Errorf("decode synthetic spec object: %w", err)
	}
	if id != "" {
		fields["id"] = id
	}
	if lifecycleState != "" {
		fields["lifecycleState"] = lifecycleState
	}
	for key, value := range extra {
		fields[key] = value
	}
	observed, err := json.Marshal(fields)
	if err != nil {
		return "", fmt.Errorf("marshal synthetic observed body: %w", err)
	}
	return string(observed), nil
}

// SDKSyntheticSession owns either a strict replay cassette or an explicitly
// requested synthetic cassette refresh.
type SDKSyntheticSession struct {
	baseClient common.BaseClient
	replay     *SDKReplaySession
	record     *SDKRecordSession
	dispatcher *syntheticResponseDispatcher
}

// OpenSDKSynthetic opens a checked-in synthetic cassette for strict replay.
// Its opt-in record mode never calls OCI; it captures requests emitted by the
// normal SDK client while serving the contract-authored response sequence.
func OpenSDKSynthetic(options SDKSyntheticOptions) (*SDKSyntheticSession, error) {
	if options.Metadata.Provenance != ProvenanceSynthetic {
		return nil, fmt.Errorf("synthetic SDK session requires synthetic provenance")
	}
	if !environmentBoolean(syntheticRecordEnv) {
		replay, err := OpenSDKReplay(SDKReplayOptions{
			Path:         options.Path,
			Host:         options.Host,
			BasePath:     options.BasePath,
			Metadata:     options.Metadata,
			Bindings:     options.Bindings,
			MaxBodyBytes: options.MaxBodyBytes,
		})
		if err != nil {
			return nil, err
		}
		return &SDKSyntheticSession{baseClient: replay.BaseClient(), replay: replay}, nil
	}

	if len(options.Responses) == 0 && options.Responder == nil {
		return nil, fmt.Errorf("synthetic SDK recording requires responses or a responder")
	}
	if len(options.Responses) != 0 && options.Responder != nil {
		return nil, fmt.Errorf("synthetic SDK recording accepts responses or a responder, not both")
	}
	host, err := replayHost(options.Host)
	if err != nil {
		return nil, err
	}
	dispatcher := &syntheticResponseDispatcher{
		responses: append([]SyntheticResponse(nil), options.Responses...),
		responder: options.Responder,
	}
	baseClient := common.DefaultBaseClientWithSigner(unsignedReplaySigner{})
	baseClient.Host = host
	baseClient.BasePath = strings.Trim(options.BasePath, "/")
	baseClient.HTTPClient = dispatcher
	record, err := OpenSDKRecord(SDKRecordOptions{
		Path:         options.Path,
		Metadata:     options.Metadata,
		BaseClient:   &baseClient,
		Bindings:     options.Bindings,
		Overwrite:    RecordingOverwriteRequested(),
		MaxBodyBytes: options.MaxBodyBytes,
	})
	if err != nil {
		return nil, err
	}
	return &SDKSyntheticSession{baseClient: baseClient, record: record, dispatcher: dispatcher}, nil
}

// NewSyntheticCRUDResponder creates a stateful cassette-authoring responder
// for a synchronous resource with standard collection and item endpoints.
func NewSyntheticCRUDResponder(options SyntheticCRUDOptions) SyntheticResponder {
	created := false
	deleted := false
	collectionPath := strings.TrimSuffix(options.CollectionPath, "/")
	return func(request *http.Request) (SyntheticResponse, error) {
		requestPath := strings.TrimSuffix(request.URL.Path, "/")
		collection := collectionPath != "" && requestPath == collectionPath
		switch request.Method {
		case http.MethodPost:
			if collectionPath == "" {
				collectionPath = requestPath
			}
			created = true
			deleted = false
			return SyntheticResponse{StatusCode: defaultHTTPStatus(options.CreateStatus, http.StatusCreated), Body: options.CreatedBody}, nil
		case http.MethodPut, http.MethodPatch:
			body := options.UpdatedBody
			if body == "" {
				body = options.CreatedBody
			}
			return SyntheticResponse{StatusCode: defaultHTTPStatus(options.UpdateStatus, http.StatusOK), Body: body}, nil
		case http.MethodDelete:
			deleted = true
			return SyntheticResponse{StatusCode: defaultHTTPStatus(options.DeleteStatus, http.StatusNoContent)}, nil
		case http.MethodGet:
			if !created {
				body := options.EmptyCollectionBody
				if body == "" {
					body = `{"items":[]}`
				}
				return SyntheticResponse{StatusCode: http.StatusOK, Body: body}, nil
			}
			if collection {
				emptyBody := options.EmptyCollectionBody
				if emptyBody == "" {
					emptyBody = `{"items":[]}`
				}
				if !created || deleted {
					return SyntheticResponse{StatusCode: http.StatusOK, Body: emptyBody}, nil
				}
				if options.PresentCollectionBody != "" {
					return SyntheticResponse{StatusCode: http.StatusOK, Body: options.PresentCollectionBody}, nil
				}
				return SyntheticResponse{StatusCode: http.StatusOK, Body: emptyBody}, nil
			}
			if deleted {
				code := options.NotFoundCode
				if code == "" {
					code = "NotFound"
				}
				return SyntheticResponse{
					StatusCode: http.StatusNotFound,
					Body:       fmt.Sprintf(`{"code":%q,"message":"resource deleted"}`, code),
				}, nil
			}
			return SyntheticResponse{StatusCode: http.StatusOK, Body: options.CreatedBody}, nil
		default:
			return SyntheticResponse{}, fmt.Errorf("unsupported synthetic CRUD request %s %s", request.Method, request.URL.String())
		}
	}
}

func defaultHTTPStatus(configured int, fallback int) int {
	if configured != 0 {
		return configured
	}
	return fallback
}

// BaseClient returns the SDK base client attached to the session.
func (s *SDKSyntheticSession) BaseClient() common.BaseClient {
	if s == nil {
		return common.BaseClient{}
	}
	return s.baseClient
}

// Close verifies replay consumption or atomically writes a fully consumed
// synthetic response sequence.
func (s *SDKSyntheticSession) Close() error {
	if s == nil {
		return nil
	}
	if s.replay != nil {
		return s.replay.Close()
	}
	if s.record == nil || s.dispatcher == nil {
		return fmt.Errorf("synthetic SDK session is not initialized")
	}
	if err := s.dispatcher.assertConsumed(); err != nil {
		return err
	}
	return s.record.Close()
}

func environmentBoolean(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

type syntheticResponseDispatcher struct {
	mu        sync.Mutex
	responses []SyntheticResponse
	responder SyntheticResponder
	next      int
	requests  []string
}

func (d *syntheticResponseDispatcher) Do(request *http.Request) (*http.Response, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.requests = append(d.requests, request.Method+" "+request.URL.String())
	if d.responder != nil {
		configured, err := d.responder(request)
		return syntheticHTTPResponse(request, configured, err)
	}
	if d.next >= len(d.responses) {
		return nil, fmt.Errorf("synthetic OCI response sequence exhausted at %s %s after requests %q", request.Method, request.URL.String(), d.requests)
	}
	configured := d.responses[d.next]
	d.next++
	return syntheticHTTPResponse(request, configured, nil)
}

func syntheticHTTPResponse(request *http.Request, configured SyntheticResponse, configuredErr error) (*http.Response, error) {
	if configuredErr != nil {
		return nil, configuredErr
	}
	statusCode := configured.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	headers := configured.Headers.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	if configured.Body != "" && headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", "application/json")
	}
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     headers,
		Body:       io.NopCloser(bytes.NewBufferString(configured.Body)),
		Request:    request,
	}, nil
}

func (d *syntheticResponseDispatcher) assertConsumed() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.responder != nil {
		if len(d.requests) == 0 {
			return fmt.Errorf("synthetic OCI responder received no requests")
		}
		return nil
	}
	if d.next != len(d.responses) {
		return fmt.Errorf("synthetic OCI response sequence consumed %d of %d responses", d.next, len(d.responses))
	}
	return nil
}
