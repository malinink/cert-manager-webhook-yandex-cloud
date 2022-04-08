# cert-manager-webhook-yandex-cloud
[![Build Status](https://travis-ci.org/malinink/cert-manager-webhook-yandex-cloud.svg?branch=master)](https://travis-ci.org/malinink/cert-manager-webhook-yandex-cloud)

Cert-manager ACME DNS webhook provider for Yandex Cloud.

## Installing

To install with helm, run:

```bash
$ git clone https://github.com/malinink/cert-manager-webhook-yandex-cloud.git
$ cd cert-manager-webhook-yandex-cloud/deploy/cert-manager-webhook-yandex-cloud
$ helm install -n cert-manager cert-manager-webhook-yandex-cloud .
```

### Issuer/ClusterIssuer

In Cloud Create `Service Account` with `Roles`: `dns.editor`
Create `Authorized Key` (To request IAM Tokens).

Apply manifest into kubernetes, an example ClusterIssuer:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: yandex-cloud-authorized-key
type: Opaque
stringData:
  serviceAccountId: __YANDEX-SERVICE-ACCOUNT-ID__
  id: __YANDEX-AUTHORIZED-KEY-ID__
  encryption: RSA_4096
  publicKey: |-
    -----BEGIN PUBLIC KEY-----
    __KEY-DATA__
    -----END PUBLIC KEY-----
  privateKey: |-
    -----BEGIN PRIVATE KEY-----
    __KEY-DATA__
    -----END PRIVATE KEY-----

---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-dns-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: mail@malinink.ru
    privateKeySecretRef:
      name: letsencrypt-dns-prod
    solvers:
      - dns01:
          webhook:
            groupName: acme.cloud.yandex.ru
            solverName: yandex-cloud
            config:
              dnsZoneId: __YANDEX-DNS-ZONE-ID__
              authorizationKeySecretName: yandex-cloud-authorized-key

```

And then you can issue a cert:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: whildcard-malinink
spec:
  secretName: whildcard-malinink-tls
  commonName: malinink.ru
  issuerRef:
    name: letsencrypt-dns-prod
    kind: ClusterIssuer
  dnsNames:
    - malinink.ru
    - "*.malinink.ru"
```

## Development

### Running the test suite

You can run the test suite with:

1. Fill in the appropriate values in `testdata/apikey.yaml` and `testdata/config.json` by `apikey.test.yaml` and `config.test.json` examples

```bash
$ RESOLVED_ZONE_NAME=example.com. make test
```
