# Security

Security controls included:

- Scoped controller RBAC for CRDs and managed child resources
- Read-only secret access for referenced registry and provider credentials
- Benchmark Jobs use a generated namespace-local ServiceAccount, Role, and RoleBinding limited to result ConfigMap writes
- NetworkPolicy examples for inference ingress, Prometheus scraping, and Qdrant isolation
- Optional cert-manager ClusterIssuer example for TLS-capable Ingresses
- Container security contexts drop Linux capabilities and disallow privilege escalation

Secret usage:

- `modelRegistrySecretName` becomes an image pull secret
- `apiTokenSecretName` and `providerKeySecretName` become `envFrom` Secret references

No credential value is stored in the CRD or generated ConfigMaps.
