/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ImageSpec defines the container image configuration
type ImageSpec struct {
	// Repository is the container image repository
	// +kubebuilder:validation:Required
	Repository string `json:"repository"`

	// Tag is the container image tag. The "latest" tag is rejected so that every
	// deployment is reproducible.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self != 'latest'",message="the 'latest' tag is not allowed; pin a version"
	Tag string `json:"tag"`

	// PullPolicy describes a policy for if/when to pull a container image
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +optional
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

// ProbeSpec defines health check probe configuration
type ProbeSpec struct {
	// Enabled indicates whether this probe is enabled
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Path is the HTTP path for the probe
	// +optional
	Path string `json:"path,omitempty"`

	// InitialDelaySeconds is the number of seconds after the container has started before probes are initiated
	// +kubebuilder:validation:Minimum=0
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`

	// PeriodSeconds specifies how often (in seconds) to perform the probe
	// +kubebuilder:validation:Minimum=1
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`

	// TimeoutSeconds specifies the number of seconds after which the probe times out
	// +kubebuilder:validation:Minimum=1
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// SuccessThreshold is the minimum consecutive successes for the probe to be considered successful
	// +kubebuilder:validation:Minimum=1
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`

	// FailureThreshold is the minimum consecutive failures for the probe to be considered failed
	// +kubebuilder:validation:Minimum=1
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
}

// HealthCheckSpec defines health check configuration
type HealthCheckSpec struct {
	// Liveness probe configuration
	// +optional
	Liveness *ProbeSpec `json:"liveness,omitempty"`

	// Readiness probe configuration
	// +optional
	Readiness *ProbeSpec `json:"readiness,omitempty"`
}

// ResourceRequirements describes the compute resource requirements
type ResourceRequirements struct {
	// Requests describes the minimum amount of compute resources required
	// +optional
	Requests ResourceList `json:"requests,omitempty"`

	// Limits describes the maximum amount of compute resources allowed
	// +optional
	Limits ResourceList `json:"limits,omitempty"`
}

// ResourceList is a set of (resource name, quantity) pairs
type ResourceList struct {
	// Memory is the memory resource requirement
	// +optional
	Memory *resource.Quantity `json:"memory,omitempty"`

	// CPU is the CPU resource requirement
	// +optional
	CPU *resource.Quantity `json:"cpu,omitempty"`
}

// ServiceSpec defines the Service configuration
type ServiceSpec struct {
	// Type is the type of Service (ClusterIP, NodePort, LoadBalancer)
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	// Port is the port that the service will expose
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`

	// TargetPort is the name or number of the port to access on the pods
	// +optional
	TargetPort string `json:"targetPort,omitempty"`
}

// RolloutSpec configures how updates are rolled out to the managed Deployment.
//
// When a readiness probe is enabled the operator defaults to a zero-downtime
// rolling update (maxSurge 1, maxUnavailable 0). Without a readiness probe it
// falls back to the Kubernetes default of 25% for both.
type RolloutSpec struct {
	// MaxSurge is the maximum number of pods that can be created above the desired
	// replica count during an update: an absolute number or a percentage such as "25%".
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`

	// MaxUnavailable is the maximum number of pods that can be unavailable during an
	// update: an absolute number or a percentage such as "25%".
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// MinReadySeconds is the minimum number of seconds a new pod must be ready,
	// without any container crashing, before it is considered available.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`

	// ProgressDeadlineSeconds is how long a rollout may make no progress before the
	// WebApi is reported as Degraded. Defaults to 600.
	// +kubebuilder:validation:Minimum=1
	// +optional
	ProgressDeadlineSeconds *int32 `json:"progressDeadlineSeconds,omitempty"`
}

// WebApiSpec defines the desired state of WebApi
//
// The XValidation rule below rejects specs that enable a probe without a
// containerPort, because probes target the named container port "http".
// +kubebuilder:validation:XValidation:rule="has(self.containerPort) || !has(self.healthCheck) || !((has(self.healthCheck.liveness) && has(self.healthCheck.liveness.enabled) && self.healthCheck.liveness.enabled) || (has(self.healthCheck.readiness) && has(self.healthCheck.readiness.enabled) && self.healthCheck.readiness.enabled))",message="containerPort is required when a probe is enabled"
type WebApiSpec struct {
	// Image specifies the container image configuration
	// +kubebuilder:validation:Required
	Image ImageSpec `json:"image"`

	// Replicas is the number of desired pods
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// ContainerPort is the port that the container exposes
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	ContainerPort int32 `json:"containerPort,omitempty"`

	// HealthCheck defines health check probes
	// +optional
	HealthCheck *HealthCheckSpec `json:"healthCheck,omitempty"`

	// Resources defines resource requirements
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// Env is a list of environment variables
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// EnvFrom is a list of sources to populate environment variables
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// Service defines the Service configuration
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// Rollout configures how updates are rolled out
	// +optional
	Rollout *RolloutSpec `json:"rollout,omitempty"`

	// Labels are additional labels to apply to all resources
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// WebApiStatus defines the observed state of WebApi.
type WebApiStatus struct {
	// Conditions represent the current state of the WebApi resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Condition types:
	// - "Ready": all desired replicas are updated, available and ready
	// - "Progressing": true only while a rollout is in flight
	// - "Degraded": the rollout failed (progress deadline exceeded) or pods cannot be created
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// DeploymentName is the name of the managed Deployment
	// +optional
	DeploymentName string `json:"deploymentName,omitempty"`

	// ServiceName is the name of the managed Service
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed WebApi
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Replicas is the total number of pods
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of pods that are ready
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image.repository`
// +kubebuilder:printcolumn:name="Tag",type=string,JSONPath=`.spec.image.tag`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.replicas`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[?(@.type=='Ready')].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// WebApi is the Schema for the webapis API
type WebApi struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of WebApi
	// +required
	Spec WebApiSpec `json:"spec"`

	// status defines the observed state of WebApi
	// +optional
	Status WebApiStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// WebApiList contains a list of WebApi
type WebApiList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []WebApi `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WebApi{}, &WebApiList{})
}
