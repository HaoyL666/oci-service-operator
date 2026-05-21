/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package genericartifactcontentbypath

import (
	"context"
	"fmt"

	genericartifactscontentv1beta1 "github.com/oracle/oci-service-operator/api/genericartifactscontent/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	"k8s.io/apimachinery/pkg/runtime"
)

type genericArtifactContentByPathDeleteResultClient interface {
	DeleteWithResult(context.Context, *genericartifactscontentv1beta1.GenericArtifactContentByPath) (servicemanager.OSOKDeleteResult, error)
}

var _ servicemanager.OSOKDeleteResultProvider = (*GenericArtifactContentByPathServiceManager)(nil)

func (c *GenericArtifactContentByPathServiceManager) DeleteWithResult(ctx context.Context, obj runtime.Object) (servicemanager.OSOKDeleteResult, error) {
	resource, err := c.convert(obj)
	if err != nil {
		c.Log.ErrorLog(err, "Conversion of object failed")
		return servicemanager.OSOKDeleteResult{}, err
	}
	if client, ok := c.client.(genericArtifactContentByPathDeleteResultClient); ok {
		return client.DeleteWithResult(ctx, resource)
	}
	deleted, err := c.client.Delete(ctx, resource)
	if err != nil {
		return servicemanager.OSOKDeleteResult{}, fmt.Errorf("delete GenericArtifactContentByPath with result: %w", err)
	}
	return servicemanager.OSOKDeleteResult{Deleted: deleted}, nil
}
