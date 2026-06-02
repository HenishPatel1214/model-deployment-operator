// Package v1alpha1 contains API Schema definitions for the ai v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=ai.platform.dev
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// SchemeGroupVersion is group version used to register these objects.
	SchemeGroupVersion = schema.GroupVersion{Group: "ai.platform.dev", Version: "v1alpha1"}

	// SchemeBuilder registers API types.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds all types of this clientset into the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion, &ModelDeployment{}, &ModelDeploymentList{})
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
