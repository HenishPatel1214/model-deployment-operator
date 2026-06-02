package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	InferenceRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "model_deployment_requests_total",
		Help: "Total inference requests observed or reported by benchmark jobs.",
	}, []string{"namespace", "modeldeployment", "provider"})

	InferenceErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "model_deployment_errors_total",
		Help: "Total inference errors observed or reported by benchmark jobs.",
	}, []string{"namespace", "modeldeployment", "provider"})

	InferenceLatencyMs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "model_deployment_latency_ms",
		Help: "Last reported average inference latency in milliseconds.",
	}, []string{"namespace", "modeldeployment", "provider"})

	TokensPerSecond = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "model_deployment_tokens_per_second",
		Help: "Last reported token throughput.",
	}, []string{"namespace", "modeldeployment", "provider"})

	BenchmarkDurationSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "model_deployment_benchmark_duration_seconds",
		Help: "Last benchmark duration in seconds.",
	}, []string{"namespace", "modeldeployment", "provider"})

	ReadyReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "model_deployment_ready_replicas",
		Help: "Ready replicas for each ModelDeployment.",
	}, []string{"namespace", "modeldeployment", "provider"})

	ModelDeploymentStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "model_deployment_status",
		Help: "Current phase encoded as a labeled gauge. The active phase is set to 1.",
	}, []string{"namespace", "modeldeployment", "provider", "phase"})

	ReconcileErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "model_deployment_reconcile_errors_total",
		Help: "Total reconcile errors emitted by the model deployment controller.",
	}, []string{"namespace", "modeldeployment", "provider", "reason"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(
		InferenceRequestsTotal,
		InferenceErrorsTotal,
		InferenceLatencyMs,
		TokensPerSecond,
		BenchmarkDurationSeconds,
		ReadyReplicas,
		ModelDeploymentStatus,
		ReconcileErrorsTotal,
	)
}

func SetPhase(namespace, name, provider, activePhase string) {
	for _, phase := range []string{"Pending", "Running", "Degraded", "Scaling", "Failed"} {
		value := 0.0
		if phase == activePhase {
			value = 1
		}
		ModelDeploymentStatus.WithLabelValues(namespace, name, provider, phase).Set(value)
	}
}
