package manager

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewCredentialClientUsesAPIReaderForSecretReads(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "credential", Namespace: "default"},
		Data:       map[string][]byte{"key": []byte("value")},
	}
	apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret.DeepCopy()).Build()
	cachedClient := &staleSecretGetClient{
		Client: apiReader,
		getErr: apierrors.NewNotFound(corev1.Resource("secrets"), secret.Name),
	}

	credClient := newCredentialClient(&fakeCredentialClientManager{
		client:    cachedClient,
		apiReader: apiReader,
	}, nil)

	data, err := credClient.GetSecret(context.Background(), secret.Name, secret.Namespace)
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}
	if !reflect.DeepEqual(data, secret.Data) {
		t.Fatalf("GetSecret() data = %v, want %v", data, secret.Data)
	}
	if cachedClient.getCalls != 0 {
		t.Fatalf("cached client Get calls = %d, want 0", cachedClient.getCalls)
	}
}

type staleSecretGetClient struct {
	ctrlclient.Client
	getCalls int
	getErr   error
}

func (c *staleSecretGetClient) Get(ctx context.Context, key ctrlclient.ObjectKey, obj ctrlclient.Object, opts ...ctrlclient.GetOption) error {
	c.getCalls++
	return c.getErr
}

type fakeCredentialClientManager struct {
	client    ctrlclient.Client
	apiReader ctrlclient.Reader
}

func (m *fakeCredentialClientManager) GetClient() ctrlclient.Client { return m.client }

func (m *fakeCredentialClientManager) GetAPIReader() ctrlclient.Reader { return m.apiReader }
