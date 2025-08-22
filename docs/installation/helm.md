## Installation

The [Crow CI Helm chart](https://codeberg.org/crowci/helm) can be installed as follows:

```bash
helm install crow oci://codeberg.org/crowci/crow
```

If you do not like the OCI variant, you can use the classical Helm repository:

```bash
helm repo add crowci https://codeberg.org/api/packages/crowci/helm
helm install crow crowci/crow
```

## Special values

### createAgentSecret

The chart uses `createAgentSecret: true` by default.
This setting creates a default agent secret which is injected into both server and agent pods.
With this, the default agent(s) provisioned by the chart are authenticated.

### CROW_BACKEND_K8S_VOLUME_SIZE

The server env var `CROW_BACKEND_K8S_VOLUME_SIZE` should be set to a low value, like `2Gi` (`1Gi` is usually too small for certain workloads).
This value determines the size of the temporary volume attached to each workflow pod.

/// tip
Perform a check if your underlying storage space is sufficient if X workflows are started in parallel.
Additionally it is recommended to create a new storage class and set the reclaim policy to "Delete", so that these temporary volumes are deleted after the workflow is finished.
///

## Public GRPC ingress

If you want to allow external agents (from users or orgs) to register at the server, you need an TLS-enabled GRPC ingress.
Here is an example configuration, which uses `ingress-nginx` and an Letsencrypt DNS issuer:

```yaml
server:
  env:
    CROW_GRPC_SECURE: "true"

ingress:
 grpc:
   enabled: true
   annotations:
     cert-manager.io/cluster-issuer: 'letsencrypt-dns01-prod'
     kubernetes.io/ingress.class: nginx
   hosts:
     - host: grpc.example.com
       paths:
         - path: /
   tls:
     - hosts:
         - grpc.grpc.example.com
       secretName: grpc.grpc.example.com-tls
```

## Agent affinity

You can increase the replica count of the agents and ensure that the pods are placed on different nodes via the following `affinity` settings:

```yaml
affinity:
 podAntiAffinity:
   requiredDuringSchedulingIgnoredDuringExecution:
     - labelSelector:
         matchExpressions:
           - key: app.kubernetes.io/name
             operator: In
             values:
               - agent
       topologyKey: kubernetes.io/hostname
```

## Sensitive secrets

Sensitive config values, like database credentials, can be provided via k8s secrets through the `extraSecretNamesForEnvFrom` field:

```yaml
extraSecretNamesForEnvFrom:
  - sensitive-secrets
```

The secret key should be the name of the env var which should be passed.
Here are some examples of sensitive values which you might wanna inject through this setting:

- `CROW_DATABASE_DATASOURCE`
- `CROW_FORGEJO_SECRET` (OAuth2)
- `CROW_AGENT_SECRET` (only if you do not use the automatically provided one by the chart)

## Metrics

To enable metrics gathering, set the following in `values.yml`:

```yaml
metrics:
  enabled: true
  port: 9001
```

This activates the `/metrics` endpoint on port 9001 without authentication.
This port is not exposed externally by default.
Use the instructions at Prometheus if you want to enable authenticated external access to metrics.

To additionally enable pod monitoring through Prometheus, set:

```yaml
prometheus:
  podmonitor:
    enabled: true
    interval: 60s
    labels: {}
```

If you are not receiving metrics after following the steps above, verify that your Prometheus configuration includes your namespace explicitly in the `podMonitorNamespaceSelector` and verify no selectors are in place:

```yaml
# Search all available namespaces
podMonitorNamespaceSelector:
  matchLabels: {}
# Enable all available pod monitors
podMonitorSelector:
  matchLabels: {}
```
