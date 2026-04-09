# Kubernetes Controller Upstream References

Use these upstream references as the authoritative source for conventions:

## Official Documentation and Guides

- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [OpenShift Conventions](https://github.com/openshift/enhancements/blob/master/CONVENTIONS.md)
- [Kubebuilder Book](https://book.kubebuilder.io/)
- [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [controller-tools](https://github.com/kubernetes-sigs/controller-tools)
- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Kubernetes Logging Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md)
- [Structured Logging Migration](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/migration-to-structured-logging.md)
- [client-go](https://github.com/kubernetes/client-go)
- [Server-Side Apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/)
- [Controller Development Pitfalls](https://ahmet.im/blog/controller-pitfalls/)
- [CRD Generation Pitfalls](https://ahmet.im/blog/crd-generation-pitfalls/)

## Reference Implementations

- [cluster-api](https://github.com/kubernetes-sigs/cluster-api) - declarative cluster lifecycle management, strong example of multi-provider architecture ([docs](https://cluster-api.sigs.k8s.io/))
- [cluster-api-operator](https://github.com/kubernetes-sigs/cluster-api-operator) - operator that manages CAPI providers, good pattern for operator-of-operators ([docs](https://cluster-api-operator.sigs.k8s.io/))
- [cluster-version-operator](https://github.com/openshift/cluster-version-operator) - manages OpenShift upgrades, example of release payload reconciliation
- [cloudnative-pg](https://github.com/cloudnative-pg/cloudnative-pg) - PostgreSQL operator, well-structured single-resource operator with status handling
- [hypershift](https://github.com/openshift/hypershift) - hosted control planes, complex multi-cluster controller patterns
