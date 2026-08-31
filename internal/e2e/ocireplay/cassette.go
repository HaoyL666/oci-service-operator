/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/common"
	"gopkg.in/yaml.v3"
)

// Cassette is both an OCI HTTP dispatcher and the lifecycle for one recording.
// It is safe for concurrent controller reconciles.
type Cassette struct {
	mu       sync.Mutex
	mode     Mode
	path     string
	delegate interface {
		Do(*http.Request) (*http.Response, error)
	}
	overwrite    bool
	maxBodyBytes int64
	file         cassetteFile
	used         []bool
	sanitizer    *sanitizer
	closed       bool
}

// Open loads a replay cassette or prepares a new recording.
func Open(options Options) (*Cassette, error) {
	if options.Path == "" {
		return nil, errors.New("cassette path is required")
	}
	if options.Mode != ModeRecord && options.Mode != ModeReplay {
		return nil, fmt.Errorf("unsupported cassette mode %q", options.Mode)
	}
	if options.Mode == ModeRecord && options.Delegate == nil {
		return nil, errors.New("record mode requires a delegate HTTP dispatcher")
	}
	if options.Metadata != nil {
		if err := validateMetadata(*options.Metadata); err != nil {
			return nil, err
		}
	}
	if options.MaxBodyBytes <= 0 {
		options.MaxBodyBytes = DefaultMaxBodyBytes
	}

	cassette := &Cassette{
		mode:         options.Mode,
		path:         options.Path,
		delegate:     options.Delegate,
		overwrite:    options.Overwrite,
		maxBodyBytes: options.MaxBodyBytes,
		file:         cassetteFile{Version: Version, Metadata: cloneMetadata(options.Metadata)},
		sanitizer:    newSanitizer(),
	}
	if options.Mode == ModeRecord {
		if !options.Overwrite {
			if _, err := os.Stat(options.Path); err == nil {
				return nil, fmt.Errorf("cassette %s already exists; remove it or enable overwrite explicitly", options.Path)
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("inspect cassette path: %w", err)
			}
		}
		return cassette, nil
	}

	content, err := os.ReadFile(options.Path)
	if err != nil {
		return nil, fmt.Errorf("read cassette: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cassette.file); err != nil {
		return nil, fmt.Errorf("decode cassette: %w", err)
	}
	if cassette.file.Version != Version {
		return nil, fmt.Errorf("cassette version %d is unsupported; expected %d", cassette.file.Version, Version)
	}
	if cassette.file.Metadata != nil {
		if err := validateMetadata(*cassette.file.Metadata); err != nil {
			return nil, err
		}
	}
	if options.Metadata != nil {
		if cassette.file.Metadata == nil {
			return nil, errors.New("replay cassette does not declare metadata")
		}
		if !metadataEqual(*options.Metadata, *cassette.file.Metadata) {
			return nil, fmt.Errorf("replay cassette metadata does not match expected metadata")
		}
	}
	if len(cassette.file.Interactions) == 0 {
		return nil, errors.New("replay cassette contains no interactions")
	}
	cassette.used = make([]bool, len(cassette.file.Interactions))
	return cassette, nil
}

// Metadata returns a defensive copy of the cassette metadata. Legacy cassettes
// without metadata return nil.
func (c *Cassette) Metadata() *Metadata {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneMetadata(c.file.Metadata)
}

// Attach replaces an OCI SDK base client's HTTP dispatcher with the cassette.
func (c *Cassette) Attach(client *common.BaseClient) {
	if c == nil || client == nil {
		return
	}
	client.HTTPClient = c
}

// Do implements common.HTTPRequestDispatcher.
func (c *Cassette) Do(req *http.Request) (*http.Response, error) {
	if c == nil {
		return nil, errors.New("nil OCI cassette")
	}
	if req == nil {
		return nil, errors.New("nil HTTP request")
	}
	requestBody, err := readAndRestoreBody(&req.Body, c.maxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("capture request body: %w", err)
	}

	switch c.mode {
	case ModeReplay:
		return c.replay(req, requestBody)
	case ModeRecord:
		return c.record(req, requestBody)
	default:
		return nil, fmt.Errorf("unsupported cassette mode %q", c.mode)
	}
}

func (c *Cassette) replay(req *http.Request, body []byte) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("OCI cassette is closed")
	}

	actual, err := c.sanitizer.request(req, body)
	if err != nil {
		return nil, err
	}
	for index, candidate := range c.file.Interactions {
		if c.used[index] || !reflect.DeepEqual(candidate.Request, actual) {
			continue
		}
		c.used[index] = true
		return replayResponse(req, candidate.Response, c.sanitizer)
	}
	return nil, fmt.Errorf("no unused OCI interaction matches %s %s%s; headers=%v body=%s", actual.Method, actual.Path, querySuffix(actual.Query), actual.Headers, actual.Body)
}

func (c *Cassette) record(req *http.Request, requestBody []byte) (*http.Response, error) {
	response, dispatchErr := c.delegate.Do(req)
	var responseBody []byte
	var captureErr error
	if response != nil {
		responseBody, captureErr = readAndRestoreBody(&response.Body, c.maxBodyBytes)
	}
	if captureErr != nil {
		return response, fmt.Errorf("capture response body: %w", captureErr)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return response, errors.New("OCI cassette is closed")
	}
	recordedRequest, err := c.sanitizer.request(req, requestBody)
	if err != nil {
		return response, err
	}
	recordedResponse, err := c.sanitizer.response(response, responseBody, dispatchErr)
	if err != nil {
		return response, err
	}
	c.file.Interactions = append(c.file.Interactions, interaction{Request: recordedRequest, Response: recordedResponse})
	return response, dispatchErr
}

// AssertConsumed fails when replay completed without exercising every recorded
// interaction. Record mode has nothing to assert.
func (c *Cassette) AssertConsumed() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.assertConsumedLocked()
}

func (c *Cassette) assertConsumedLocked() error {
	if c.mode != ModeReplay {
		return nil
	}
	var unused []string
	for index, wasUsed := range c.used {
		if wasUsed {
			continue
		}
		request := c.file.Interactions[index].Request
		unused = append(unused, fmt.Sprintf("%d:%s %s%s", index+1, request.Method, request.Path, querySuffix(request.Query)))
	}
	if len(unused) > 0 {
		return fmt.Errorf("OCI cassette has %d unused interaction(s): %s", len(unused), strings.Join(unused, ", "))
	}
	return nil
}

// Close verifies replay consumption or atomically persists a new recording.
func (c *Cassette) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.mode == ModeReplay {
		return c.assertConsumedLocked()
	}
	return c.writeLocked()
}

func (c *Cassette) writeLocked() error {
	if len(c.file.Interactions) == 0 {
		return errors.New("refusing to write an empty OCI cassette")
	}
	if c.file.Metadata != nil {
		if err := validateMetadata(*c.file.Metadata); err != nil {
			return err
		}
	}
	content, err := yaml.Marshal(c.file)
	if err != nil {
		return fmt.Errorf("encode cassette: %w", err)
	}
	if err := validateSafeRecording(content); err != nil {
		return err
	}

	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create cassette directory: %w", err)
	}
	temporary, err := os.CreateTemp(dir, "."+filepath.Base(c.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary cassette: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary cassette: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync temporary cassette: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary cassette: %w", err)
	}
	if err := os.Chmod(temporaryPath, 0o644); err != nil {
		return fmt.Errorf("set cassette permissions: %w", err)
	}
	if !c.overwrite {
		if _, err := os.Stat(c.path); err == nil {
			return fmt.Errorf("cassette %s appeared while recording; refusing to overwrite it", c.path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect cassette before rename: %w", err)
		}
	}
	if err := os.Rename(temporaryPath, c.path); err != nil {
		return fmt.Errorf("publish cassette atomically: %w", err)
	}
	return nil
}

func replayResponse(request *http.Request, recorded recordedResponse, sanitizer *sanitizer) (*http.Response, error) {
	body, err := decodeBody(recorded.Body, recorded.Encoding)
	if err != nil {
		return nil, err
	}
	if recorded.Error != "" {
		return nil, errors.New(recorded.Error)
	}
	if recorded.StatusCode < 100 || recorded.StatusCode > 599 {
		return nil, fmt.Errorf("invalid replay status code %d", recorded.StatusCode)
	}
	header := make(http.Header, len(recorded.Headers))
	for name, values := range recorded.Headers {
		for _, value := range values {
			header.Add(name, sanitizer.restoreText(value))
		}
	}
	if recorded.Encoding == "json" || recorded.Encoding == "text" || recorded.Encoding == "" {
		body = []byte(sanitizer.restoreText(string(body)))
	}
	return &http.Response{
		StatusCode:    recorded.StatusCode,
		Status:        fmt.Sprintf("%d %s", recorded.StatusCode, http.StatusText(recorded.StatusCode)),
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       request,
	}, nil
}

func readAndRestoreBody(body *io.ReadCloser, limit int64) ([]byte, error) {
	if body == nil || *body == nil || *body == http.NoBody {
		return nil, nil
	}
	content, err := io.ReadAll(io.LimitReader(*body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		_ = (*body).Close()
		*body = http.NoBody
		return nil, fmt.Errorf("body exceeds %d byte cassette limit", limit)
	}
	if err := (*body).Close(); err != nil {
		return nil, err
	}
	*body = io.NopCloser(bytes.NewReader(content))
	return content, nil
}

func querySuffix(query string) string {
	if query == "" {
		return ""
	}
	return "?" + query
}

func validateSafeRecording(content []byte) error {
	lower := strings.ToLower(string(content))
	for _, marker := range []string{
		"-----begin private key-----",
		"-----begin rsa private key-----",
		"security_token=",
		"authorization: signature",
	} {
		if strings.Contains(lower, marker) {
			return fmt.Errorf("refusing to persist cassette containing sensitive marker %q", marker)
		}
	}
	return nil
}
