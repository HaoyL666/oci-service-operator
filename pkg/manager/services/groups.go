package services

import (
	"fmt"

	"github.com/oracle/oci-service-operator/internal/registrations"
	"github.com/oracle/oci-service-operator/pkg/manager"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	ctrl "sigs.k8s.io/controller-runtime"
)

func registerGroup(group string) manager.RegisterFunc {
	return func(mgr ctrl.Manager, deps *manager.Dependencies) error {
		registration, ok := registrations.ByGroup(group)
		if !ok {
			return fmt.Errorf("manager services: registration for group %q not found", group)
		}

		ctx := registrations.NewContext(mgr, servicemanager.RuntimeDeps{
			Provider:         deps.Provider,
			CredentialClient: deps.CredClient,
			Scheme:           deps.Scheme,
			Metrics:          deps.Metrics,
		})
		if err := registration.SetupWithManager(ctx); err != nil {
			return err
		}

		for _, webhook := range registrations.ManualWebhooksByGroup(group) {
			if err := webhook.SetupWithManager(mgr); err != nil {
				return err
			}
		}

		return nil
	}
}

// Database registers the database group with the shared manager.
func Database() manager.RegisterFunc {
	return registerGroup("database")
}

// MySQL registers the mysql group with the shared manager.
func MySQL() manager.RegisterFunc {
	return registerGroup("mysql")
}

// Streaming registers the streaming group with the shared manager.
func Streaming() manager.RegisterFunc {
	return registerGroup("streaming")
}
