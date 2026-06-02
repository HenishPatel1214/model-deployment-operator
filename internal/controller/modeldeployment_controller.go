package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	aiv1alpha1 "github.com/HenishPatel1214/model-deployment-operator/api/v1alpha1"
	mdmetrics "github.com/HenishPatel1214/model-deployment-operator/internal/metrics"
	"github.com/HenishPatel1214/model-deployment-operator/internal/resources"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const ModelDeploymentFinalizer = "modeldeployment.ai.platform.dev/finalizer"

type ModelDeploymentReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=ai.platform.dev,resources=modeldeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ai.platform.dev,resources=modeldeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ai.platform.dev,resources=modeldeployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;serviceaccounts;events;pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
func (r *ModelDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var md aiv1alpha1.ModelDeployment
	if err := r.Get(ctx, req.NamespacedName, &md); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !md.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&md, ModelDeploymentFinalizer) {
			if err := r.finalize(ctx, &md); err != nil {
				mdmetrics.ReconcileErrorsTotal.WithLabelValues(md.Namespace, md.Name, resources.Provider(&md), "finalize").Inc()
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(&md, ModelDeploymentFinalizer)
			return ctrl.Result{}, r.Update(ctx, &md)
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&md, ModelDeploymentFinalizer) {
		controllerutil.AddFinalizer(&md, ModelDeploymentFinalizer)
		if err := r.Update(ctx, &md); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if err := r.reconcileDesiredState(ctx, &md); err != nil {
		logger.Error(err, "failed to reconcile desired state")
		r.recordWarning(&md, "ReconcileFailed", err.Error())
		mdmetrics.ReconcileErrorsTotal.WithLabelValues(md.Namespace, md.Name, resources.Provider(&md), "apply").Inc()
		_ = r.updateFailureStatus(ctx, &md, "ReconcileFailed", err.Error())
		return ctrl.Result{}, err
	}

	if err := r.updateObservedStatus(ctx, &md); err != nil {
		mdmetrics.ReconcileErrorsTotal.WithLabelValues(md.Namespace, md.Name, resources.Provider(&md), "status").Inc()
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *ModelDeploymentReconciler) reconcileDesiredState(ctx context.Context, md *aiv1alpha1.ModelDeployment) error {
	runtimeConfig := resources.BuildRuntimeConfigMap(md)
	if err := r.apply(ctx, md, runtimeConfig); err != nil {
		return err
	}

	costReport, err := resources.BuildCostReportConfigMap(md)
	if err != nil {
		return err
	}
	if err := r.apply(ctx, md, costReport); err != nil {
		return err
	}

	deployment, err := resources.BuildDeployment(md)
	if err != nil {
		return err
	}
	if err := r.apply(ctx, md, deployment); err != nil {
		return err
	}

	if err := r.apply(ctx, md, resources.BuildService(md)); err != nil {
		return err
	}

	if md.Spec.Ingress.Enabled {
		if err := r.apply(ctx, md, resources.BuildIngress(md)); err != nil {
			return err
		}
	} else if err := r.deleteOwned(ctx, &networkingv1.Ingress{}, md.Namespace, resources.IngressName(md)); err != nil {
		return err
	}

	if md.Spec.Autoscaling.Enabled {
		if err := r.apply(ctx, md, resources.BuildHPA(md)); err != nil {
			return err
		}
	} else if err := r.deleteOwned(ctx, &autoscalingv2.HorizontalPodAutoscaler{}, md.Namespace, resources.HPAName(md)); err != nil {
		return err
	}

	if md.Spec.Benchmark.Enabled {
		if err := r.apply(ctx, md, resources.BuildBenchmarkResultConfigMap(md)); err != nil {
			return err
		}
		if err := r.apply(ctx, md, resources.BuildBenchmarkServiceAccount(md)); err != nil {
			return err
		}
		if err := r.apply(ctx, md, resources.BuildBenchmarkRole(md)); err != nil {
			return err
		}
		if err := r.apply(ctx, md, resources.BuildBenchmarkRoleBinding(md)); err != nil {
			return err
		}
		if err := r.apply(ctx, md, resources.BuildBenchmarkJob(md)); err != nil {
			return err
		}
	} else if err := r.deleteOwned(ctx, &batchv1.Job{}, md.Namespace, resources.BenchmarkJobName(md)); err != nil {
		return err
	} else if err := r.deleteOwned(ctx, &corev1.ConfigMap{}, md.Namespace, resources.BenchmarkResultConfigMapName(md)); err != nil {
		return err
	} else if err := r.deleteOwned(ctx, &corev1.ServiceAccount{}, md.Namespace, resources.BenchmarkServiceAccountName(md)); err != nil {
		return err
	} else if err := r.deleteOwned(ctx, &rbacv1.Role{}, md.Namespace, resources.BenchmarkServiceAccountName(md)); err != nil {
		return err
	} else if err := r.deleteOwned(ctx, &rbacv1.RoleBinding{}, md.Namespace, resources.BenchmarkServiceAccountName(md)); err != nil {
		return err
	}

	if md.Spec.RAG.Enabled {
		if err := r.apply(ctx, md, resources.BuildQdrantDeployment(md)); err != nil {
			return err
		}
		if err := r.apply(ctx, md, resources.BuildQdrantService(md)); err != nil {
			return err
		}
	} else {
		if err := r.deleteOwned(ctx, &appsv1.Deployment{}, md.Namespace, resources.QdrantName(md)); err != nil {
			return err
		}
		if err := r.deleteOwned(ctx, &corev1.Service{}, md.Namespace, resources.QdrantName(md)); err != nil {
			return err
		}
	}

	return nil
}

func (r *ModelDeploymentReconciler) apply(ctx context.Context, owner *aiv1alpha1.ModelDeployment, desired client.Object) error {
	if err := ctrl.SetControllerReference(owner, desired, r.Scheme); err != nil {
		return err
	}

	existing := desired.DeepCopyObject().(client.Object)
	key := client.ObjectKeyFromObject(desired)
	if err := r.Get(ctx, key, existing); err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, desired)
		}
		return err
	}

	switch desiredObj := desired.(type) {
	case *corev1.Service:
		existingSvc := existing.(*corev1.Service)
		desiredObj.Spec.ClusterIP = existingSvc.Spec.ClusterIP
		desiredObj.Spec.ClusterIPs = existingSvc.Spec.ClusterIPs
		desiredObj.Spec.IPFamilies = existingSvc.Spec.IPFamilies
		desiredObj.Spec.IPFamilyPolicy = existingSvc.Spec.IPFamilyPolicy
		desiredObj.Spec.HealthCheckNodePort = existingSvc.Spec.HealthCheckNodePort
	case *batchv1.Job:
		return nil
	}

	desired.SetResourceVersion(existing.GetResourceVersion())
	return r.Update(ctx, desired)
}

func (r *ModelDeploymentReconciler) updateObservedStatus(ctx context.Context, md *aiv1alpha1.ModelDeployment) error {
	next := md.Status.DeepCopy()
	next.Endpoint = resources.Endpoint(md)
	next.Phase = aiv1alpha1.ModelDeploymentPhasePending
	next.ReadyReplicas = 0

	var deployment appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: resources.DeploymentName(md), Namespace: md.Namespace}, &deployment); err != nil {
		if apierrors.IsNotFound(err) {
			setCondition(next, md.Generation, "Ready", metav1.ConditionFalse, "DeploymentMissing", "Inference Deployment has not been created yet")
			next.Phase = aiv1alpha1.ModelDeploymentPhasePending
			return r.patchStatus(ctx, md, next)
		}
		return err
	}

	next.ReadyReplicas = deployment.Status.ReadyReplicas
	if reason, message, failed := r.detectPodFailure(ctx, md); failed {
		next.Phase = aiv1alpha1.ModelDeploymentPhaseFailed
		setCondition(next, md.Generation, "Ready", metav1.ConditionFalse, reason, message)
		r.recordWarning(md, reason, message)
	} else if isDeploymentAvailable(&deployment) {
		next.Phase = aiv1alpha1.ModelDeploymentPhaseRunning
		setCondition(next, md.Generation, "Ready", metav1.ConditionTrue, "DeploymentAvailable", "Model deployment is running")
	} else if md.Spec.Autoscaling.Enabled && deployment.Status.ReadyReplicas < deployment.Status.Replicas && deployment.Status.Replicas > resources.Replicas(md) {
		next.Phase = aiv1alpha1.ModelDeploymentPhaseScaling
		setCondition(next, md.Generation, "Ready", metav1.ConditionFalse, "Scaling", "Deployment is scaling toward the HPA target")
	} else {
		next.Phase = aiv1alpha1.ModelDeploymentPhaseDegraded
		setCondition(next, md.Generation, "Ready", metav1.ConditionFalse, "DeploymentUnavailable", "Deployment is not yet available")
	}

	if md.Spec.Benchmark.Enabled {
		r.observeBenchmark(ctx, md, next)
	}

	mdmetrics.ReadyReplicas.WithLabelValues(md.Namespace, md.Name, resources.Provider(md)).Set(float64(next.ReadyReplicas))
	mdmetrics.SetPhase(md.Namespace, md.Name, resources.Provider(md), string(next.Phase))
	if next.LastBenchmark != nil {
		mdmetrics.BenchmarkDurationSeconds.WithLabelValues(md.Namespace, md.Name, resources.Provider(md)).Set(float64(next.LastBenchmark.DurationSeconds))
		mdmetrics.InferenceLatencyMs.WithLabelValues(md.Namespace, md.Name, resources.Provider(md)).Set(float64(next.LastBenchmark.AverageLatencyMs))
		mdmetrics.TokensPerSecond.WithLabelValues(md.Namespace, md.Name, resources.Provider(md)).Set(next.LastBenchmark.TokensPerSecond)
	}

	return r.patchStatus(ctx, md, next)
}

func (r *ModelDeploymentReconciler) observeBenchmark(ctx context.Context, md *aiv1alpha1.ModelDeployment, status *aiv1alpha1.ModelDeploymentStatus) {
	var job batchv1.Job
	if err := r.Get(ctx, types.NamespacedName{Name: resources.BenchmarkJobName(md), Namespace: md.Namespace}, &job); err != nil {
		if apierrors.IsNotFound(err) {
			setCondition(status, md.Generation, "BenchmarkReady", metav1.ConditionFalse, "BenchmarkJobMissing", "Benchmark Job has not been created")
		}
		return
	}

	switch {
	case job.Status.Failed > 0:
		setCondition(status, md.Generation, "BenchmarkReady", metav1.ConditionFalse, "BenchmarkFailed", "Benchmark Job failed; inspect pod logs for the JSON result or error")
	case job.Status.Succeeded > 0:
		result := r.readBenchmarkResult(ctx, md)
		if result == nil {
			result = &aiv1alpha1.BenchmarkResult{
				RequestCount: md.Spec.Benchmark.Requests,
				ErrorCount:   0,
				ErrorRate:    0,
			}
			if job.Status.StartTime != nil && job.Status.CompletionTime != nil {
				result.DurationSeconds = int64(job.Status.CompletionTime.Sub(job.Status.StartTime.Time).Seconds())
			}
		}
		status.LastBenchmark = result
		setCondition(status, md.Generation, "BenchmarkReady", metav1.ConditionTrue, "BenchmarkCompleted", "Benchmark Job completed and results were recorded")
	default:
		setCondition(status, md.Generation, "BenchmarkReady", metav1.ConditionFalse, "BenchmarkRunning", "Benchmark Job is running")
	}
}

func (r *ModelDeploymentReconciler) readBenchmarkResult(ctx context.Context, md *aiv1alpha1.ModelDeployment) *aiv1alpha1.BenchmarkResult {
	var cm corev1.ConfigMap
	if err := r.Get(ctx, types.NamespacedName{Name: resources.BenchmarkResultConfigMapName(md), Namespace: md.Namespace}, &cm); err != nil {
		return nil
	}
	raw := cm.Data["result.json"]
	if raw == "" {
		return nil
	}
	var payload struct {
		AverageLatencyMs int64   `json:"averageLatencyMs"`
		P95LatencyMs     int64   `json:"p95LatencyMs"`
		TokensPerSecond  float64 `json:"tokensPerSecond"`
		Requests         int32   `json:"requests"`
		ErrorCount       int32   `json:"errorCount"`
		ErrorRate        float64 `json:"errorRate"`
		DurationSeconds  int64   `json:"durationSeconds"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return &aiv1alpha1.BenchmarkResult{
		AverageLatencyMs: payload.AverageLatencyMs,
		P95LatencyMs:     payload.P95LatencyMs,
		TokensPerSecond:  payload.TokensPerSecond,
		RequestCount:     payload.Requests,
		ErrorCount:       payload.ErrorCount,
		ErrorRate:        payload.ErrorRate,
		DurationSeconds:  payload.DurationSeconds,
	}
}

func (r *ModelDeploymentReconciler) updateFailureStatus(ctx context.Context, md *aiv1alpha1.ModelDeployment, reason, message string) error {
	next := md.Status.DeepCopy()
	next.Phase = aiv1alpha1.ModelDeploymentPhaseFailed
	setCondition(next, md.Generation, "Ready", metav1.ConditionFalse, reason, message)
	return r.patchStatus(ctx, md, next)
}

func (r *ModelDeploymentReconciler) patchStatus(ctx context.Context, md *aiv1alpha1.ModelDeployment, next *aiv1alpha1.ModelDeploymentStatus) error {
	if equality.Semantic.DeepEqual(md.Status, *next) {
		return nil
	}
	md.Status = *next
	return r.Status().Update(ctx, md)
}

func (r *ModelDeploymentReconciler) detectPodFailure(ctx context.Context, md *aiv1alpha1.ModelDeployment) (string, string, bool) {
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(md.Namespace), client.MatchingLabels(resources.SelectorLabels(md))); err != nil {
		return "", "", false
	}

	for _, pod := range pods.Items {
		for _, status := range pod.Status.ContainerStatuses {
			if status.State.Waiting != nil {
				reason := status.State.Waiting.Reason
				switch reason {
				case "ImagePullBackOff", "ErrImagePull", "CrashLoopBackOff", "CreateContainerConfigError":
					return reason, fmt.Sprintf("Pod %s container %s is waiting: %s", pod.Name, status.Name, status.State.Waiting.Message), true
				}
			}
			if status.LastTerminationState.Terminated != nil && status.LastTerminationState.Terminated.Reason == "OOMKilled" {
				return "OOMKilled", fmt.Sprintf("Pod %s container %s exceeded its memory limit", pod.Name, status.Name), true
			}
		}
	}
	return "", "", false
}

func isDeploymentAvailable(deployment *appsv1.Deployment) bool {
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentAvailable && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func setCondition(status *aiv1alpha1.ModelDeploymentStatus, observedGeneration int64, conditionType string, conditionStatus metav1.ConditionStatus, reason, message string) {
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		ObservedGeneration: observedGeneration,
		Reason:             reason,
		Message:            message,
	})
}

func (r *ModelDeploymentReconciler) finalize(ctx context.Context, md *aiv1alpha1.ModelDeployment) error {
	children := []client.Object{
		&appsv1.Deployment{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&networkingv1.Ingress{},
		&autoscalingv2.HorizontalPodAutoscaler{},
		&batchv1.Job{},
		&corev1.ConfigMap{},
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
	}

	names := []string{
		resources.DeploymentName(md),
		resources.ServiceName(md),
		resources.IngressName(md),
		resources.HPAName(md),
		resources.BenchmarkJobName(md),
		resources.BenchmarkServiceAccountName(md),
		resources.RuntimeConfigMapName(md),
		resources.CostReportConfigMapName(md),
		resources.BenchmarkResultConfigMapName(md),
		resources.QdrantName(md),
	}

	for _, obj := range children {
		for _, name := range names {
			if err := r.deleteOwned(ctx, obj.DeepCopyObject().(client.Object), md.Namespace, name); err != nil {
				return err
			}
		}
	}
	r.recordNormal(md, "Finalized", "Verified cleanup of operator-managed resources")
	return nil
}

func (r *ModelDeploymentReconciler) deleteOwned(ctx context.Context, obj client.Object, namespace, name string) error {
	obj.SetNamespace(namespace)
	obj.SetName(name)
	if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *ModelDeploymentReconciler) recordNormal(md *aiv1alpha1.ModelDeployment, reason, message string) {
	if r.Recorder != nil {
		r.Recorder.Event(md, corev1.EventTypeNormal, reason, message)
	}
}

func (r *ModelDeploymentReconciler) recordWarning(md *aiv1alpha1.ModelDeployment, reason, message string) {
	if r.Recorder != nil {
		r.Recorder.Event(md, corev1.EventTypeWarning, reason, message)
	}
}

func (r *ModelDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor("modeldeployment-controller")
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&aiv1alpha1.ModelDeployment{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}
