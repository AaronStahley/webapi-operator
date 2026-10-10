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

package controller

import (
	"context"
	"fmt"
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1alpha1 "github.com/AaronStahley/webapi-operator/api/v1alpha1"
)

// WebApiReconciler reconciles a WebApi object
type WebApiReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	NamespaceSelector labels.Selector
}

// +kubebuilder:rbac:groups=platform.aaronstahley.dev,resources=webapis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.aaronstahley.dev,resources=webapis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.aaronstahley.dev,resources=webapis/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *WebApiReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the WebApi instance
	webApi := &apiv1alpha1.WebApi{}
	if err := r.Get(ctx, req.NamespacedName, webApi); err != nil {
		if errors.IsNotFound(err) {
			// WebApi not found, could have been deleted
			logger.Info("WebApi resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get WebApi")
		return ctrl.Result{}, err
	}

	// If a namespace selector is configured, skip namespaces that don't match
	if r.NamespaceSelector != nil && !r.NamespaceSelector.Empty() {
		ns := &corev1.Namespace{}
		if err := r.Get(ctx, types.NamespacedName{Name: req.Namespace}, ns); err != nil {
			if errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
		if !r.NamespaceSelector.Matches(labels.Set(ns.Labels)) {
			return ctrl.Result{}, nil
		}
	}

	// Check if the resource is being deleted
	if !webApi.DeletionTimestamp.IsZero() {
		// Resource is being deleted, cleanup handled by owner references
		return ctrl.Result{}, nil
	}

	// Reconcile Deployment
	deployment := &appsv1.Deployment{}
	deploymentName := webApi.Name
	err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: webApi.Namespace}, deployment)
	if err != nil && errors.IsNotFound(err) {
		// Create new Deployment
		dep := r.buildDeployment(webApi)
		logger.Info("Creating Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		if err := r.Create(ctx, dep); err != nil {
			logger.Error(err, "Failed to create Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully, requeue to check status
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	} else {
		// Update existing Deployment only if the fields we manage have changed.
		dep := r.buildDeployment(webApi)
		updateReason := deploymentDrift(deployment, dep)
		needsUpdate := updateReason != ""
		if needsUpdate {
			deployment.Spec = dep.Spec
			deployment.Labels = dep.Labels
			deployment.Annotations = dep.Annotations
			logger.Info("Updating Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name, "reason", updateReason)
			if err := r.Update(ctx, deployment); err != nil {
				logger.Error(err, "Failed to update Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
				return ctrl.Result{}, err
			}
		}
	}

	// Reconcile Service
	service := &corev1.Service{}
	serviceName := webApi.Name
	err = r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: webApi.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		// Create new Service
		svc := r.buildService(webApi)
		logger.Info("Creating Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		if err := r.Create(ctx, svc); err != nil {
			logger.Error(err, "Failed to create Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
		// Service created successfully, requeue to check status
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	} else {
		// Update existing Service only if the fields we manage have changed.
		svc := r.buildService(webApi)
		svcNeedsUpdate := service.Spec.Type != svc.Spec.Type ||
			!equality.Semantic.DeepEqual(service.Spec.Selector, svc.Spec.Selector) ||
			!equality.Semantic.DeepEqual(service.Labels, svc.Labels)
		if !svcNeedsUpdate && len(service.Spec.Ports) > 0 && len(svc.Spec.Ports) > 0 {
			ap := service.Spec.Ports[0]
			dp := svc.Spec.Ports[0]
			svcNeedsUpdate = ap.Port != dp.Port ||
				ap.TargetPort != dp.TargetPort ||
				ap.Protocol != dp.Protocol
		}
		if svcNeedsUpdate {
			service.Spec.Type = svc.Spec.Type
			service.Spec.Ports = svc.Spec.Ports
			service.Spec.Selector = svc.Spec.Selector
			service.Labels = svc.Labels
			service.Annotations = svc.Annotations
			logger.Info("Updating Service", "Service.Namespace", service.Namespace, "Service.Name", service.Name)
			if err := r.Update(ctx, service); err != nil {
				logger.Error(err, "Failed to update Service", "Service.Namespace", service.Namespace, "Service.Name", service.Name)
				return ctrl.Result{}, err
			}
		}
	}

	// Update WebApi status
	if err := r.updateStatus(ctx, webApi, deployment); err != nil {
		logger.Error(err, "Failed to update WebApi status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// deploymentDrift returns the first managed field that differs between the actual and
// desired Deployment, or "" if none do. Comparing the full Spec would always differ due
// to Kubernetes-added defaults (terminationMessagePath, dnsPolicy, etc.).
func deploymentDrift(deployment, dep *appsv1.Deployment) string {
	if !equality.Semantic.DeepEqual(deployment.Spec.Replicas, dep.Spec.Replicas) {
		return "replicas"
	} else if !equality.Semantic.DeepEqual(deployment.Spec.Template.Labels, dep.Spec.Template.Labels) {
		return "templateLabels"
	} else if !equality.Semantic.DeepEqual(deployment.Labels, dep.Labels) {
		return "labels"
	} else if !equality.Semantic.DeepEqual(deployment.Spec.Strategy, dep.Spec.Strategy) {
		return "strategy"
	} else if deployment.Spec.MinReadySeconds != dep.Spec.MinReadySeconds {
		return "minReadySeconds"
	} else if !equality.Semantic.DeepEqual(deployment.Spec.ProgressDeadlineSeconds, dep.Spec.ProgressDeadlineSeconds) {
		return "progressDeadlineSeconds"
	} else if len(deployment.Spec.Template.Spec.Containers) > 0 && len(dep.Spec.Template.Spec.Containers) > 0 {
		actual := deployment.Spec.Template.Spec.Containers[0]
		desired := dep.Spec.Template.Spec.Containers[0]
		if actual.Image != desired.Image {
			return "image"
		} else if actual.ImagePullPolicy != desired.ImagePullPolicy {
			return "imagePullPolicy"
		} else if !equality.Semantic.DeepEqual(actual.Env, desired.Env) {
			return "env"
		} else if !equality.Semantic.DeepEqual(actual.EnvFrom, desired.EnvFrom) {
			return "envFrom"
		} else if !equality.Semantic.DeepEqual(actual.Ports, desired.Ports) {
			return "ports"
		} else if !equality.Semantic.DeepEqual(actual.Resources, desired.Resources) {
			return "resources"
		} else if !equality.Semantic.DeepEqual(actual.LivenessProbe, desired.LivenessProbe) {
			return "livenessProbe"
		} else if !equality.Semantic.DeepEqual(actual.ReadinessProbe, desired.ReadinessProbe) {
			return "readinessProbe"
		}
	}
	return ""
}

// commonLabels returns the labels applied to every object owned by a WebApi.
func commonLabels(webApi *apiv1alpha1.WebApi) map[string]string {
	l := map[string]string{
		"app":                          webApi.Name,
		"app.kubernetes.io/name":       webApi.Name,
		"app.kubernetes.io/managed-by": "webapi-operator",
	}
	maps.Copy(l, webApi.Spec.Labels)
	return l
}

// buildDeployment creates a Deployment from the WebApi spec
func (r *WebApiReconciler) buildDeployment(webApi *apiv1alpha1.WebApi) *appsv1.Deployment {
	replicas := int32(1)
	if webApi.Spec.Replicas != nil {
		replicas = *webApi.Spec.Replicas
	}

	objLabels := commonLabels(webApi)

	// Build container spec
	container := corev1.Container{
		Name:            webApi.Name,
		Image:           fmt.Sprintf("%s:%s", webApi.Spec.Image.Repository, webApi.Spec.Image.Tag),
		ImagePullPolicy: webApi.Spec.Image.PullPolicy,
		Env:             webApi.Spec.Env,
		EnvFrom:         webApi.Spec.EnvFrom,
	}

	// Add container port
	if webApi.Spec.ContainerPort > 0 {
		container.Ports = []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: webApi.Spec.ContainerPort,
				Protocol:      corev1.ProtocolTCP,
			},
		}
	}

	// Add resource requirements
	if webApi.Spec.Resources != nil {
		resources := corev1.ResourceRequirements{}

		// Add requests
		if webApi.Spec.Resources.Requests.Memory != nil || webApi.Spec.Resources.Requests.CPU != nil {
			resources.Requests = corev1.ResourceList{}
			if webApi.Spec.Resources.Requests.Memory != nil {
				resources.Requests[corev1.ResourceMemory] = *webApi.Spec.Resources.Requests.Memory
			}
			if webApi.Spec.Resources.Requests.CPU != nil {
				resources.Requests[corev1.ResourceCPU] = *webApi.Spec.Resources.Requests.CPU
			}
		}

		// Add limits
		if webApi.Spec.Resources.Limits.Memory != nil || webApi.Spec.Resources.Limits.CPU != nil {
			resources.Limits = corev1.ResourceList{}
			if webApi.Spec.Resources.Limits.Memory != nil {
				resources.Limits[corev1.ResourceMemory] = *webApi.Spec.Resources.Limits.Memory
			}
			if webApi.Spec.Resources.Limits.CPU != nil {
				resources.Limits[corev1.ResourceCPU] = *webApi.Spec.Resources.Limits.CPU
			}
		}

		container.Resources = resources
	}

	// Add health probes
	if webApi.Spec.HealthCheck != nil {
		if webApi.Spec.HealthCheck.Liveness != nil && webApi.Spec.HealthCheck.Liveness.Enabled {
			container.LivenessProbe = &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   webApi.Spec.HealthCheck.Liveness.Path,
						Port:   intstr.FromString("http"),
						Scheme: corev1.URISchemeHTTP,
					},
				},
				InitialDelaySeconds: webApi.Spec.HealthCheck.Liveness.InitialDelaySeconds,
				PeriodSeconds:       webApi.Spec.HealthCheck.Liveness.PeriodSeconds,
				TimeoutSeconds:      webApi.Spec.HealthCheck.Liveness.TimeoutSeconds,
				SuccessThreshold:    webApi.Spec.HealthCheck.Liveness.SuccessThreshold,
				FailureThreshold:    webApi.Spec.HealthCheck.Liveness.FailureThreshold,
			}
		}
		if webApi.Spec.HealthCheck.Readiness != nil && webApi.Spec.HealthCheck.Readiness.Enabled {
			container.ReadinessProbe = &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   webApi.Spec.HealthCheck.Readiness.Path,
						Port:   intstr.FromString("http"),
						Scheme: corev1.URISchemeHTTP,
					},
				},
				InitialDelaySeconds: webApi.Spec.HealthCheck.Readiness.InitialDelaySeconds,
				PeriodSeconds:       webApi.Spec.HealthCheck.Readiness.PeriodSeconds,
				TimeoutSeconds:      webApi.Spec.HealthCheck.Readiness.TimeoutSeconds,
				SuccessThreshold:    webApi.Spec.HealthCheck.Readiness.SuccessThreshold,
				FailureThreshold:    webApi.Spec.HealthCheck.Readiness.FailureThreshold,
			}
		}
	}

	rollout := effectiveRollout(webApi)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      webApi.Name,
			Namespace: webApi.Namespace,
			Labels:    objLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &rollout.maxSurge,
					MaxUnavailable: &rollout.maxUnavailable,
				},
			},
			MinReadySeconds:         rollout.minReadySeconds,
			ProgressDeadlineSeconds: &rollout.progressDeadlineSeconds,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": webApi.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: objLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
				},
			},
		},
	}

	// Set WebApi as the owner and controller
	_ = controllerutil.SetControllerReference(webApi, deployment, r.Scheme)

	return deployment
}

// buildService creates a Service from the WebApi spec
func (r *WebApiReconciler) buildService(webApi *apiv1alpha1.WebApi) *corev1.Service {
	objLabels := commonLabels(webApi)

	serviceType := corev1.ServiceTypeClusterIP
	port := int32(80)
	targetPort := intstr.FromString("http")

	if webApi.Spec.Service != nil {
		if webApi.Spec.Service.Type != "" {
			serviceType = webApi.Spec.Service.Type
		}
		if webApi.Spec.Service.Port > 0 {
			port = webApi.Spec.Service.Port
		}
		if webApi.Spec.Service.TargetPort != "" {
			targetPort = intstr.FromString(webApi.Spec.Service.TargetPort)
		}
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      webApi.Name,
			Namespace: webApi.Namespace,
			Labels:    objLabels,
		},
		Spec: corev1.ServiceSpec{
			Type: serviceType,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: targetPort,
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": webApi.Name,
			},
		},
	}

	// Set WebApi as the owner and controller
	_ = controllerutil.SetControllerReference(webApi, service, r.Scheme)

	return service
}

// updateStatus updates the WebApi status based on the current Deployment state
func (r *WebApiReconciler) updateStatus(ctx context.Context, webApi *apiv1alpha1.WebApi, deployment *appsv1.Deployment) error {
	// Update status fields
	webApi.Status.DeploymentName = deployment.Name
	webApi.Status.ServiceName = webApi.Name
	webApi.Status.ObservedGeneration = webApi.Generation
	webApi.Status.Replicas = deployment.Status.Replicas
	webApi.Status.ReadyReplicas = deployment.Status.ReadyReplicas

	// Update conditions (Ready, Progressing, Degraded) from the Deployment's rollout state.
	// SetStatusCondition only bumps LastTransitionTime when a condition's status changes.
	for _, c := range buildConditions(webApi, deployment) {
		meta.SetStatusCondition(&webApi.Status.Conditions, c)
	}

	// Update the status
	if err := r.Status().Update(ctx, webApi); err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WebApiReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.WebApi{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("webapi").
		Complete(r)
}
