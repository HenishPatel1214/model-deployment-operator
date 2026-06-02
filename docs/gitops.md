# GitOps

The Helm chart is under:

```text
charts/model-deployment-operator
```

Install with Helm:

```bash
helm upgrade --install model-deployment-operator charts/model-deployment-operator \
  --namespace model-deployment-operator-system \
  --create-namespace
```

Install with Argo CD:

```bash
kubectl apply -f gitops/argocd/application.yaml
```

The Argo CD Application uses automated sync, prune, self-heal, and namespace creation. Update `spec.source.repoURL` if the repository is pushed under a different owner.
