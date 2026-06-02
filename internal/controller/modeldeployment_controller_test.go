package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	aiv1alpha1 "github.com/HenishPatel1214/model-deployment-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func TestModelDeploymentReconcileCreatesResources(t *testing.T) {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("KUBEBUILDER_ASSETS is not set; run make test to install envtest assets")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheme := runtime.NewScheme()
	must(t, clientgoscheme.AddToScheme(scheme))
	must(t, aiv1alpha1.AddToScheme(scheme))
	must(t, appsv1.AddToScheme(scheme))
	must(t, autoscalingv2.AddToScheme(scheme))
	must(t, batchv1.AddToScheme(scheme))
	must(t, networkingv1.AddToScheme(scheme))
	must(t, rbacv1.AddToScheme(scheme))

	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
	}
	cfg, err := testEnv.Start()
	must(t, err)
	defer func() {
		if err := testEnv.Stop(); err != nil {
			t.Logf("envtest cleanup warning: %v", err)
		}
	}()

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
	})
	must(t, err)
	must(t, (&ModelDeploymentReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr))

	go func() {
		if err := mgr.Start(ctx); err != nil {
			t.Errorf("manager exited: %v", err)
		}
	}()

	k8sClient := mgr.GetClient()
	ensureNamespace(t, ctx, k8sClient, "default")

	md := &aiv1alpha1.ModelDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-demo", Namespace: "default"},
		Spec: aiv1alpha1.ModelDeploymentSpec{
			Provider:  "ollama",
			ModelName: "llama3",
			Image:     "ollama/ollama:latest",
			Replicas:  1,
			Resources: aiv1alpha1.ResourceSpec{
				CPU:    "500m",
				Memory: "1Gi",
			},
			Service: aiv1alpha1.ServiceSpec{
				Type: "ClusterIP",
				Port: 11434,
			},
			Ingress: aiv1alpha1.IngressSpec{
				Enabled: true,
				Host:    "llama.local",
			},
			Autoscaling: aiv1alpha1.AutoscalingSpec{
				Enabled:                           true,
				MinReplicas:                       1,
				MaxReplicas:                       3,
				TargetCPUUtilizationPercentage:    70,
				TargetMemoryUtilizationPercentage: 80,
			},
			Benchmark: aiv1alpha1.BenchmarkSpec{
				Enabled:      true,
				SamplePrompt: "Explain operators.",
				Requests:     3,
				Concurrency:  1,
			},
			RAG: aiv1alpha1.RAGSpec{
				Enabled: true,
				VectorDatabase: aiv1alpha1.VectorDatabaseSpec{
					Provider: "qdrant",
					Image:    "qdrant/qdrant:latest",
				},
			},
		},
	}
	must(t, k8sClient.Create(ctx, md))

	waitForObject(t, ctx, k8sClient, types.NamespacedName{Name: "llama-demo", Namespace: "default"}, &appsv1.Deployment{})
	waitForObject(t, ctx, k8sClient, types.NamespacedName{Name: "llama-demo", Namespace: "default"}, &corev1.Service{})
	waitForObject(t, ctx, k8sClient, types.NamespacedName{Name: "llama-demo", Namespace: "default"}, &networkingv1.Ingress{})
	waitForObject(t, ctx, k8sClient, types.NamespacedName{Name: "llama-demo", Namespace: "default"}, &autoscalingv2.HorizontalPodAutoscaler{})
	waitForObject(t, ctx, k8sClient, types.NamespacedName{Name: "llama-demo-benchmark", Namespace: "default"}, &batchv1.Job{})
	waitForObject(t, ctx, k8sClient, types.NamespacedName{Name: "llama-demo-benchmark", Namespace: "default"}, &corev1.ServiceAccount{})
	waitForObject(t, ctx, k8sClient, types.NamespacedName{Name: "llama-demo-benchmark", Namespace: "default"}, &rbacv1.Role{})
	waitForObject(t, ctx, k8sClient, types.NamespacedName{Name: "llama-demo-benchmark", Namespace: "default"}, &rbacv1.RoleBinding{})
	waitForObject(t, ctx, k8sClient, types.NamespacedName{Name: "llama-demo-benchmark-result", Namespace: "default"}, &corev1.ConfigMap{})
	waitForObject(t, ctx, k8sClient, types.NamespacedName{Name: "llama-demo-qdrant", Namespace: "default"}, &appsv1.Deployment{})
	waitForObject(t, ctx, k8sClient, types.NamespacedName{Name: "llama-demo-qdrant", Namespace: "default"}, &corev1.Service{})
	waitForObject(t, ctx, k8sClient, types.NamespacedName{Name: "llama-demo-cost-report", Namespace: "default"}, &corev1.ConfigMap{})

	var refreshed aiv1alpha1.ModelDeployment
	waitFor(t, func() bool {
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: "llama-demo", Namespace: "default"}, &refreshed); err != nil {
			return false
		}
		return len(refreshed.Finalizers) > 0 && refreshed.Status.Phase != ""
	}, "ModelDeployment finalizer and status")

	must(t, k8sClient.Delete(ctx, &refreshed))
	waitFor(t, func() bool {
		var dep appsv1.Deployment
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "llama-demo", Namespace: "default"}, &dep)
		return apierrors.IsNotFound(err)
	}, "owned deployment cleanup")
}

func ensureNamespace(t *testing.T, ctx context.Context, c client.Client, name string) {
	t.Helper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	err := c.Create(ctx, ns)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatal(err)
	}
}

func waitForObject(t *testing.T, ctx context.Context, c client.Client, key types.NamespacedName, obj client.Object) {
	t.Helper()
	waitFor(t, func() bool {
		return c.Get(ctx, key, obj) == nil
	}, key.String())
}

func waitFor(t *testing.T, condition func() bool, name string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", name)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
