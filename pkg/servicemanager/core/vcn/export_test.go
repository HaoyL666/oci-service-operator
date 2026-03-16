/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package vcn

import (
	ocicore "github.com/oracle/oci-go-sdk/v65/core"
	corev1beta1 "github.com/oracle/oci-service-operator/api/core/v1beta1"
)

// ExportSetClientForTest sets the OCI client on the service manager for unit testing.
func ExportSetClientForTest(m *VcnServiceManager, c VirtualNetworkClientInterface) {
	m.ociClient = c
}

// ExportBuildCreateDetailsForTest exposes buildCreateVcnDetails for assertions.
func ExportBuildCreateDetailsForTest(resource corev1beta1.Vcn) (ocicore.CreateVcnDetails, error) {
	return buildCreateVcnDetails(resource)
}
