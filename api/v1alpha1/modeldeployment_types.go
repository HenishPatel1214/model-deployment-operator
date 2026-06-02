package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ModelDeploymentPhase string

const (
	ModelDeploymentPhasePending  ModelDeploymentPhase = "Pending"
	ModelDeploymentPhaseRunning  ModelDeploymentPhase = "Running"
	ModelDeploymentPhaseDegraded ModelDeploymentPhase = "Degraded"
	ModelDeploymentPhaseScaling  ModelDeploymentPhase = "Scaling"
	ModelDeploymentPhaseFailed   ModelDeploymentPhase = "Failed"
)

// ModelDeploymentSpec defines the desired state for an AI inference workload.
type ModelDeploymentSpec struct {
	// +kubebuilder:validation:Enum=ollama;vllm
	// +kubebuilder:default=ollama
	Provider string `json:"provider,omitempty"`

	// +kubebuilder:validation:MinLength=1
	ModelName string `json:"modelName"`

	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

	Resources     ResourceSpec      `json:"resources,omitempty"`
	Service       ServiceSpec       `json:"service,omitempty"`
	Ingress       IngressSpec       `json:"ingress,omitempty"`
	Autoscaling   AutoscalingSpec   `json:"autoscaling,omitempty"`
	Observability ObservabilitySpec `json:"observability,omitempty"`
	Benchmark     BenchmarkSpec     `json:"benchmark,omitempty"`
	RAG           RAGSpec           `json:"rag,omitempty"`
	Secrets       SecretSpec        `json:"secrets,omitempty"`
}

type ResourceSpec struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
	// +kubebuilder:validation:Minimum=0
	GPU int64 `json:"gpu,omitempty"`
}

type ServiceSpec struct {
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:default=ClusterIP
	Type string `json:"type,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=11434
	Port int32 `json:"port,omitempty"`
}

type IngressSpec struct {
	Enabled bool   `json:"enabled,omitempty"`
	Host    string `json:"host,omitempty"`
	TLS     bool   `json:"tls,omitempty"`
}

type AutoscalingSpec struct {
	Enabled bool `json:"enabled,omitempty"`
	// +kubebuilder:validation:Minimum=1
	MinReplicas int32 `json:"minReplicas,omitempty"`
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetCPUUtilizationPercentage int32 `json:"targetCPUUtilizationPercentage,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetMemoryUtilizationPercentage int32 `json:"targetMemoryUtilizationPercentage,omitempty"`
}

type ObservabilitySpec struct {
	Enabled       bool `json:"enabled,omitempty"`
	Prometheus    bool `json:"prometheus,omitempty"`
	OpenTelemetry bool `json:"openTelemetry,omitempty"`
}

type BenchmarkSpec struct {
	Enabled bool `json:"enabled,omitempty"`
	// +kubebuilder:default="Explain Kubernetes operators in simple terms."
	SamplePrompt string `json:"samplePrompt,omitempty"`
	// +kubebuilder:validation:Minimum=1
	Requests int32 `json:"requests,omitempty"`
	// +kubebuilder:validation:Minimum=1
	Concurrency int32 `json:"concurrency,omitempty"`
}

type RAGSpec struct {
	Enabled        bool               `json:"enabled,omitempty"`
	VectorDatabase VectorDatabaseSpec `json:"vectorDatabase,omitempty"`
}

type VectorDatabaseSpec struct {
	// +kubebuilder:validation:Enum=qdrant
	// +kubebuilder:default=qdrant
	Provider string `json:"provider,omitempty"`
	Image    string `json:"image,omitempty"`
}

type SecretSpec struct {
	ModelRegistrySecretName string `json:"modelRegistrySecretName,omitempty"`
	APITokenSecretName      string `json:"apiTokenSecretName,omitempty"`
	ProviderKeySecretName   string `json:"providerKeySecretName,omitempty"`
}

// ModelDeploymentStatus defines the observed state of ModelDeployment.
type ModelDeploymentStatus struct {
	Phase         ModelDeploymentPhase `json:"phase,omitempty"`
	ReadyReplicas int32                `json:"readyReplicas,omitempty"`
	Endpoint      string               `json:"endpoint,omitempty"`
	LastBenchmark *BenchmarkResult     `json:"lastBenchmark,omitempty"`
	Conditions    []metav1.Condition   `json:"conditions,omitempty"`
}

type BenchmarkResult struct {
	AverageLatencyMs int64   `json:"averageLatencyMs,omitempty"`
	P95LatencyMs     int64   `json:"p95LatencyMs,omitempty"`
	TokensPerSecond  float64 `json:"tokensPerSecond,omitempty"`
	RequestCount     int32   `json:"requestCount,omitempty"`
	ErrorCount       int32   `json:"errorCount,omitempty"`
	ErrorRate        float64 `json:"errorRate,omitempty"`
	DurationSeconds  int64   `json:"durationSeconds,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=md,path=modeldeployments,scope=Namespaced
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.endpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ModelDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelDeploymentSpec   `json:"spec,omitempty"`
	Status ModelDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ModelDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModelDeployment `json:"items"`
}
