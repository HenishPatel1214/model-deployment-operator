package resources

import (
	"fmt"
	"strconv"

	aiv1alpha1 "github.com/HenishPatel1214/model-deployment-operator/api/v1alpha1"
	"github.com/HenishPatel1214/model-deployment-operator/internal/cost"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	ManagedByLabel = "model-deployment-operator"
	AppName        = "model-deployment"
)

func Labels(md *aiv1alpha1.ModelDeployment, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       AppName,
		"app.kubernetes.io/instance":   md.Name,
		"app.kubernetes.io/component":  component,
		"app.kubernetes.io/managed-by": ManagedByLabel,
		"ai.platform.dev/provider":     Provider(md),
	}
}

func SelectorLabels(md *aiv1alpha1.ModelDeployment) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     AppName,
		"app.kubernetes.io/instance": md.Name,
	}
}

func Provider(md *aiv1alpha1.ModelDeployment) string {
	if md.Spec.Provider == "" {
		return "ollama"
	}
	return md.Spec.Provider
}

func ServicePort(md *aiv1alpha1.ModelDeployment) int32 {
	if md.Spec.Service.Port <= 0 {
		return 11434
	}
	return md.Spec.Service.Port
}

func Replicas(md *aiv1alpha1.ModelDeployment) int32 {
	if md.Spec.Replicas <= 0 {
		return 1
	}
	return md.Spec.Replicas
}

func DeploymentName(md *aiv1alpha1.ModelDeployment) string {
	return md.Name
}

func ServiceName(md *aiv1alpha1.ModelDeployment) string {
	return md.Name
}

func IngressName(md *aiv1alpha1.ModelDeployment) string {
	return md.Name
}

func HPAName(md *aiv1alpha1.ModelDeployment) string {
	return md.Name
}

func RuntimeConfigMapName(md *aiv1alpha1.ModelDeployment) string {
	return md.Name + "-runtime"
}

func CostReportConfigMapName(md *aiv1alpha1.ModelDeployment) string {
	return md.Name + "-cost-report"
}

func BenchmarkJobName(md *aiv1alpha1.ModelDeployment) string {
	return md.Name + "-benchmark"
}

func BenchmarkServiceAccountName(md *aiv1alpha1.ModelDeployment) string {
	return md.Name + "-benchmark"
}

func QdrantName(md *aiv1alpha1.ModelDeployment) string {
	return md.Name + "-qdrant"
}

func Endpoint(md *aiv1alpha1.ModelDeployment) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", ServiceName(md), md.Namespace, ServicePort(md))
}

func BuildDeployment(md *aiv1alpha1.ModelDeployment) (*appsv1.Deployment, error) {
	port := ServicePort(md)
	replicas := Replicas(md)
	provider := Provider(md)
	labels := Labels(md, "inference")
	selector := SelectorLabels(md)

	container := corev1.Container{
		Name:            "inference",
		Image:           md.Spec.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Ports: []corev1.ContainerPort{{
			Name:          "http",
			ContainerPort: port,
		}},
		Env: []corev1.EnvVar{
			{Name: "MODEL_NAME", Value: md.Spec.ModelName},
			{Name: "PROVIDER", Value: provider},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}

	if provider == "ollama" {
		container.Env = append(container.Env, corev1.EnvVar{Name: "OLLAMA_HOST", Value: fmt.Sprintf("0.0.0.0:%d", port)})
	}

	if provider == "vllm" {
		container.Args = []string{"--model", md.Spec.ModelName, "--host", "0.0.0.0", "--port", strconv.Itoa(int(port))}
	}

	if md.Spec.RAG.Enabled {
		container.Env = append(container.Env,
			corev1.EnvVar{Name: "RAG_ENABLED", Value: "true"},
			corev1.EnvVar{Name: "VECTOR_DATABASE_PROVIDER", Value: vectorProvider(md)},
			corev1.EnvVar{Name: "VECTOR_DATABASE_ENDPOINT", Value: fmt.Sprintf("http://%s.%s.svc.cluster.local:6333", QdrantName(md), md.Namespace)},
		)
	}

	if md.Spec.Secrets.APITokenSecretName != "" {
		container.EnvFrom = append(container.EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: md.Spec.Secrets.APITokenSecretName},
			},
		})
	}
	if md.Spec.Secrets.ProviderKeySecretName != "" {
		container.EnvFrom = append(container.EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: md.Spec.Secrets.ProviderKeySecretName},
			},
		})
	}

	if err := applyResourceRequirements(&container, md); err != nil {
		return nil, err
	}

	probePath := "/"
	if provider == "vllm" {
		probePath = "/health"
	} else if provider == "ollama" {
		probePath = "/api/tags"
	}
	container.ReadinessProbe = httpProbe(probePath, 10, 5)
	container.LivenessProbe = httpProbe(probePath, 30, 10)

	podAnnotations := map[string]string{}
	if md.Spec.Observability.Enabled && md.Spec.Observability.Prometheus {
		podAnnotations["prometheus.io/scrape"] = "true"
		podAnnotations["prometheus.io/port"] = strconv.Itoa(int(port))
		podAnnotations["prometheus.io/path"] = "/metrics"
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName(md),
			Namespace: md.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
				},
			},
		},
	}

	if md.Spec.Secrets.ModelRegistrySecretName != "" {
		dep.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: md.Spec.Secrets.ModelRegistrySecretName}}
	}

	return dep, nil
}

func BuildService(md *aiv1alpha1.ModelDeployment) *corev1.Service {
	serviceType := corev1.ServiceTypeClusterIP
	switch md.Spec.Service.Type {
	case "NodePort":
		serviceType = corev1.ServiceTypeNodePort
	case "LoadBalancer":
		serviceType = corev1.ServiceTypeLoadBalancer
	}

	port := ServicePort(md)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName(md),
			Namespace: md.Namespace,
			Labels:    Labels(md, "service"),
		},
		Spec: corev1.ServiceSpec{
			Type:     serviceType,
			Selector: SelectorLabels(md),
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       port,
				TargetPort: intstrFromInt32(port),
			}},
		},
	}
}

func BuildIngress(md *aiv1alpha1.ModelDeployment) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	host := md.Spec.Ingress.Host
	if host == "" {
		host = md.Name + ".local"
	}
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      IngressName(md),
			Namespace: md.Namespace,
			Labels:    Labels(md, "ingress"),
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: ServiceName(md),
									Port: networkingv1.ServiceBackendPort{Number: ServicePort(md)},
								},
							},
						}},
					},
				},
			}},
		},
	}
	if md.Spec.Ingress.TLS {
		ing.Spec.TLS = []networkingv1.IngressTLS{{
			Hosts:      []string{host},
			SecretName: md.Name + "-tls",
		}}
		ing.Annotations = map[string]string{"cert-manager.io/cluster-issuer": "letsencrypt-prod"}
	}
	return ing
}

func BuildHPA(md *aiv1alpha1.ModelDeployment) *autoscalingv2.HorizontalPodAutoscaler {
	minReplicas := md.Spec.Autoscaling.MinReplicas
	if minReplicas <= 0 {
		minReplicas = 1
	}
	maxReplicas := md.Spec.Autoscaling.MaxReplicas
	if maxReplicas <= 0 {
		maxReplicas = 5
	}

	metrics := []autoscalingv2.MetricSpec{}
	if md.Spec.Autoscaling.TargetCPUUtilizationPercentage > 0 {
		metrics = append(metrics, resourceMetric(corev1.ResourceCPU, md.Spec.Autoscaling.TargetCPUUtilizationPercentage))
	}
	if md.Spec.Autoscaling.TargetMemoryUtilizationPercentage > 0 {
		metrics = append(metrics, resourceMetric(corev1.ResourceMemory, md.Spec.Autoscaling.TargetMemoryUtilizationPercentage))
	}
	if len(metrics) == 0 {
		metrics = append(metrics, resourceMetric(corev1.ResourceCPU, 70))
	}

	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HPAName(md),
			Namespace: md.Namespace,
			Labels:    Labels(md, "autoscaler"),
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       DeploymentName(md),
			},
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			Metrics:     metrics,
		},
	}
}

func BuildBenchmarkJob(md *aiv1alpha1.ModelDeployment) *batchv1.Job {
	requests := md.Spec.Benchmark.Requests
	if requests <= 0 {
		requests = 20
	}
	concurrency := md.Spec.Benchmark.Concurrency
	if concurrency <= 0 {
		concurrency = 2
	}
	prompt := md.Spec.Benchmark.SamplePrompt
	if prompt == "" {
		prompt = "Explain Kubernetes operators in simple terms."
	}
	backoffLimit := int32(1)
	ttl := int32(900)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BenchmarkJobName(md),
			Namespace: md.Namespace,
			Labels:    Labels(md, "benchmark"),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: Labels(md, "benchmark")},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:            "benchmark",
						Image:           "ghcr.io/henishpatel1214/model-deployment-benchmark:latest",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env: []corev1.EnvVar{
							{Name: "MODEL_PROVIDER", Value: Provider(md)},
							{Name: "MODEL_ENDPOINT", Value: Endpoint(md)},
							{Name: "MODEL_NAME", Value: md.Spec.ModelName},
							{Name: "SAMPLE_PROMPT", Value: prompt},
							{Name: "REQUESTS", Value: strconv.Itoa(int(requests))},
							{Name: "CONCURRENCY", Value: strconv.Itoa(int(concurrency))},
							{Name: "RESULT_CONFIGMAP", Value: BenchmarkResultConfigMapName(md)},
							{
								Name: "POD_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
								},
							},
						},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: boolPtr(false),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
					}},
					ServiceAccountName: BenchmarkServiceAccountName(md),
				},
			},
		},
	}
}

func BenchmarkResultConfigMapName(md *aiv1alpha1.ModelDeployment) string {
	return md.Name + "-benchmark-result"
}

func BuildBenchmarkResultConfigMap(md *aiv1alpha1.ModelDeployment) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BenchmarkResultConfigMapName(md),
			Namespace: md.Namespace,
			Labels:    Labels(md, "benchmark"),
		},
		Data: map[string]string{
			"status": "pending",
		},
	}
}

func BuildBenchmarkServiceAccount(md *aiv1alpha1.ModelDeployment) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BenchmarkServiceAccountName(md),
			Namespace: md.Namespace,
			Labels:    Labels(md, "benchmark"),
		},
	}
}

func BuildBenchmarkRole(md *aiv1alpha1.ModelDeployment) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BenchmarkServiceAccountName(md),
			Namespace: md.Namespace,
			Labels:    Labels(md, "benchmark"),
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "create", "update", "patch"},
		}},
	}
}

func BuildBenchmarkRoleBinding(md *aiv1alpha1.ModelDeployment) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BenchmarkServiceAccountName(md),
			Namespace: md.Namespace,
			Labels:    Labels(md, "benchmark"),
		},
		Subjects: []rbacv1.Subject{{
			Kind: "ServiceAccount",
			Name: BenchmarkServiceAccountName(md),
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     BenchmarkServiceAccountName(md),
		},
	}
}

func BuildRuntimeConfigMap(md *aiv1alpha1.ModelDeployment) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RuntimeConfigMapName(md),
			Namespace: md.Namespace,
			Labels:    Labels(md, "config"),
		},
		Data: map[string]string{
			"provider":       Provider(md),
			"modelName":      md.Spec.ModelName,
			"endpoint":       Endpoint(md),
			"ragEnabled":     strconv.FormatBool(md.Spec.RAG.Enabled),
			"vectorDatabase": vectorProvider(md),
		},
	}
}

func BuildCostReportConfigMap(md *aiv1alpha1.ModelDeployment) (*corev1.ConfigMap, error) {
	report, err := cost.Estimate(md.Spec.ModelName, md.Spec.Resources.CPU, md.Spec.Resources.Memory, md.Spec.Resources.GPU, Replicas(md))
	if err != nil {
		return nil, err
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CostReportConfigMapName(md),
			Namespace: md.Namespace,
			Labels:    Labels(md, "cost-report"),
		},
		Data: map[string]string{"report.yaml": report.YAML()},
	}, nil
}

func BuildQdrantDeployment(md *aiv1alpha1.ModelDeployment) *appsv1.Deployment {
	replicas := int32(1)
	image := md.Spec.RAG.VectorDatabase.Image
	if image == "" {
		image = "qdrant/qdrant:latest"
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      QdrantName(md),
			Namespace: md.Namespace,
			Labels:    Labels(md, "vector-db"),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app.kubernetes.io/name":     "qdrant",
				"app.kubernetes.io/instance": md.Name,
			}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":       "qdrant",
						"app.kubernetes.io/instance":   md.Name,
						"app.kubernetes.io/component":  "vector-db",
						"app.kubernetes.io/managed-by": ManagedByLabel,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "qdrant",
						Image: image,
						Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 6333}},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: boolPtr(false),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
					}},
				},
			},
		},
	}
}

func BuildQdrantService(md *aiv1alpha1.ModelDeployment) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      QdrantName(md),
			Namespace: md.Namespace,
			Labels:    Labels(md, "vector-db"),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app.kubernetes.io/name":     "qdrant",
				"app.kubernetes.io/instance": md.Name,
			},
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       6333,
				TargetPort: intstrFromInt32(6333),
			}},
		},
	}
}

func applyResourceRequirements(container *corev1.Container, md *aiv1alpha1.ModelDeployment) error {
	requests := corev1.ResourceList{}
	limits := corev1.ResourceList{}

	if md.Spec.Resources.CPU != "" {
		q, err := resource.ParseQuantity(md.Spec.Resources.CPU)
		if err != nil {
			return fmt.Errorf("parse cpu resource: %w", err)
		}
		requests[corev1.ResourceCPU] = q
		limits[corev1.ResourceCPU] = q
	}

	if md.Spec.Resources.Memory != "" {
		q, err := resource.ParseQuantity(md.Spec.Resources.Memory)
		if err != nil {
			return fmt.Errorf("parse memory resource: %w", err)
		}
		requests[corev1.ResourceMemory] = q
		limits[corev1.ResourceMemory] = q
	}

	if md.Spec.Resources.GPU > 0 {
		limits[corev1.ResourceName("nvidia.com/gpu")] = resource.MustParse(strconv.FormatInt(md.Spec.Resources.GPU, 10))
	}

	if len(requests) > 0 || len(limits) > 0 {
		container.Resources = corev1.ResourceRequirements{Requests: requests, Limits: limits}
	}
	return nil
}

func resourceMetric(name corev1.ResourceName, target int32) autoscalingv2.MetricSpec {
	return autoscalingv2.MetricSpec{
		Type: autoscalingv2.ResourceMetricSourceType,
		Resource: &autoscalingv2.ResourceMetricSource{
			Name: name,
			Target: autoscalingv2.MetricTarget{
				Type:               autoscalingv2.UtilizationMetricType,
				AverageUtilization: &target,
			},
		},
	}
}

func vectorProvider(md *aiv1alpha1.ModelDeployment) string {
	if md.Spec.RAG.VectorDatabase.Provider == "" {
		return "qdrant"
	}
	return md.Spec.RAG.VectorDatabase.Provider
}

func httpProbe(path string, initialDelay, period int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstrFromString("http"),
			},
		},
		InitialDelaySeconds: initialDelay,
		PeriodSeconds:       period,
		FailureThreshold:    6,
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func intstrFromInt32(v int32) intstr.IntOrString {
	return intstr.FromInt(int(v))
}

func intstrFromString(v string) intstr.IntOrString {
	return intstr.FromString(v)
}
