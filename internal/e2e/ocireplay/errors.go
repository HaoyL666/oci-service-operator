/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"errors"
	"fmt"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// IsHTTPStatus recognizes both raw OCI SDK errors and repository-normalized
// errors while keeping retry policy in the resource-specific scenario.
func IsHTTPStatus(err error, status int) bool {
	if err == nil {
		return false
	}
	var serviceErr common.ServiceError
	if errors.As(err, &serviceErr) && serviceErr.GetHTTPStatusCode() == status {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), strings.ToLower(fmt.Sprintf("http status code: %d", status)))
}
