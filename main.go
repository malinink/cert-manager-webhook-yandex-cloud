package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/cmd"
	"github.com/malinink/cert-manager-webhook-yandex-cloud/yandex"
	v1 "k8s.io/api/core/v1"
	extAPI "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
)

const (
	providerName = "yandex-cloud"
)

var GroupName = os.Getenv("GROUP_NAME")

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	// This will register our custom DNS provider with the webhook serving
	// library, making it available as an API under the provided GroupName.
	// You can register multiple DNS provider implementations with a single
	// webhook, where the Name() method will be used to disambiguate between
	// the different implementations.
	cmd.RunWebhookServer(GroupName,
		&yandexCloudDNSProviderSolver{},
	)
}

// yandexCloudDNSProviderSolver implements the yandex-cloud-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
type yandexCloudDNSProviderSolver struct {
	Client *kubernetes.Clientset
}

// yandexCloudDNSProviderConfig is a structure that is used to decode into when
// solving a DNS01 challenge.
type yandexCloudDNSProviderConfig struct {
	DnsZoneId                  string `json:"dnsZoneId"`
	AuthorizationKeySecretName string `json:"authorizationKeySecretName"`
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *yandexCloudDNSProviderSolver) Name() string {
	return providerName
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *yandexCloudDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	yandexCloudDNSProvider, err := fetchYandexCloudDNSProvider(c, ch)

	if err != nil {
		return err
	}

	return yandexCloudDNSProvider.Present(ch.ResolvedFQDN, ch.Key)
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *yandexCloudDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	yandexCloudDNSProvider, err := fetchYandexCloudDNSProvider(c, ch)

	if err != nil {
		return err
	}

	return yandexCloudDNSProvider.CleanUp(ch.ResolvedFQDN, ch.Key)
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *yandexCloudDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	c.Client = cl

	return nil
}

func fetchYandexCloudDNSProvider(c *yandexCloudDNSProviderSolver, ch *v1alpha1.ChallengeRequest) (*yandex.YandexCloudDNSProvider, error) {

	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return nil, err
	}

	dnsZoneId := cfg.DnsZoneId
	if dnsZoneId == "" {
		return nil, fmt.Errorf("dnsZoneId field was not provided")
	}

	authorizationKeySecretName := cfg.AuthorizationKeySecretName
	if authorizationKeySecretName == "" {
		return nil, fmt.Errorf("authorizationKeySecretName field was not provided")
	}

	secret, err := c.Client.CoreV1().Secrets(ch.ResourceNamespace).Get(context.Background(), authorizationKeySecretName, metaV1.GetOptions{})

	if err != nil {
		return nil, fmt.Errorf("unable to get secret `%s/%s`; %v", authorizationKeySecretName, ch.ResourceNamespace, err)
	}

	id, err := fetchKeyFromSecret(secret, "id", authorizationKeySecretName, ch.ResourceNamespace)

	if err != nil {
		return nil, err
	}

	serviceAccountId, err := fetchKeyFromSecret(secret, "serviceAccountId", authorizationKeySecretName, ch.ResourceNamespace)

	if err != nil {
		return nil, err
	}

	encryption, err := fetchKeyFromSecret(secret, "encryption", authorizationKeySecretName, ch.ResourceNamespace)

	if err != nil {
		encryption = "RSA_2048"
	}

	publicKey, err := fetchKeyFromSecret(secret, "publicKey", authorizationKeySecretName, ch.ResourceNamespace)

	if err != nil {
		return nil, err
	}

	privateKey, err := fetchKeyFromSecret(secret, "privateKey", authorizationKeySecretName, ch.ResourceNamespace)

	if err != nil {
		return nil, err
	}

	return yandex.NewYandexCloudDNSProvider(
		dnsZoneId,
		serviceAccountId,
		id,
		encryption,
		publicKey,
		privateKey,
	)
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *extAPI.JSON) (yandexCloudDNSProviderConfig, error) {
	cfg := yandexCloudDNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}

func fetchKeyFromSecret(secret *v1.Secret, key string, authorizationKeySecretName string, resourceNamespace string) (string, error) {
	keyDataBytes, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret \"%s/%s\"",
			key,
			authorizationKeySecretName,
			resourceNamespace,
		)
	}
	return string(keyDataBytes), nil
}
