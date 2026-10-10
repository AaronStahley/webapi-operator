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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	apiv1alpha1 "github.com/AaronStahley/webapi-operator/api/v1alpha1"
)

// These are plain unit tests (no envtest), so they run with `go test -run` without a cluster.

func newWebApi(replicas int32, readiness bool) *apiv1alpha1.WebApi {
	w := &apiv1alpha1.WebApi{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 1},
		Spec:       apiv1alpha1.WebApiSpec{Replicas: &replicas},
	}
	if readiness {
		w.Spec.ContainerPort = 8080
		w.Spec.HealthCheck = &apiv1alpha1.HealthCheckSpec{
			Readiness: &apiv1alpha1.ProbeSpec{Enabled: true, Path: "/readyz"},
		}
	}
	return w
}

func TestEffectiveRolloutDefaults(t *testing.T) {
	t.Run("zero-downtime when a readiness probe is enabled", func(t *testing.T) {
		s := effectiveRollout(newWebApi(2, true))
		if s.maxSurge != intstr.FromInt32(1) || s.maxUnavailable != intstr.FromInt32(0) {
			t.Fatalf("want surge 1 / unavailable 0, got %v / %v", s.maxSurge, s.maxUnavailable)
		}
		if s.progressDeadlineSeconds != defaultProgressDeadlineSeconds {
			t.Fatalf("want default deadline %d, got %d", defaultProgressDeadlineSeconds, s.progressDeadlineSeconds)
		}
	})

	t.Run("kubernetes default without a readiness probe", func(t *testing.T) {
		s := effectiveRollout(newWebApi(2, false))
		if s.maxSurge != intstr.FromString("25%") || s.maxUnavailable != intstr.FromString("25%") {
			t.Fatalf("want 25%%/25%%, got %v / %v", s.maxSurge, s.maxUnavailable)
		}
	})
}

func TestEffectiveRolloutOverrides(t *testing.T) {
	surge := intstr.FromString("50%")
	unavailable := intstr.FromInt32(1)
	deadline := int32(120)

	w := newWebApi(3, true)
	w.Spec.Rollout = &apiv1alpha1.RolloutSpec{
		MaxSurge:                &surge,
		MaxUnavailable:          &unavailable,
		MinReadySeconds:         10,
		ProgressDeadlineSeconds: &deadline,
	}

	s := effectiveRollout(w)
	if s.maxSurge != surge || s.maxUnavailable != unavailable {
		t.Fatalf("overrides not applied: %v / %v", s.maxSurge, s.maxUnavailable)
	}
	if s.minReadySeconds != 10 || s.progressDeadlineSeconds != 120 {
		t.Fatalf("want minReady 10 / deadline 120, got %d / %d", s.minReadySeconds, s.progressDeadlineSeconds)
	}
}

func deploymentWithStatus(replicas, updated, available, ready int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
			Replicas:           replicas,
			UpdatedReplicas:    updated,
			AvailableReplicas:  available,
			ReadyReplicas:      ready,
		},
	}
}

func TestBuildConditions(t *testing.T) {
	deadlineExceeded := deploymentWithStatus(3, 1, 2, 2)
	deadlineExceeded.Status.Conditions = []appsv1.DeploymentCondition{{
		Type:    appsv1.DeploymentProgressing,
		Status:  corev1.ConditionFalse,
		Reason:  "ProgressDeadlineExceeded",
		Message: "ReplicaSet has timed out progressing.",
	}}

	replicaFailure := deploymentWithStatus(0, 0, 0, 0)
	replicaFailure.Status.Conditions = []appsv1.DeploymentCondition{{
		Type:    appsv1.DeploymentReplicaFailure,
		Status:  corev1.ConditionTrue,
		Reason:  "FailedCreate",
		Message: "exceeded quota",
	}}

	staleGeneration := deploymentWithStatus(3, 3, 3, 3)
	staleGeneration.Generation = 2 // controller has not observed the latest spec yet

	tests := []struct {
		name                                           string
		webApi                                         *apiv1alpha1.WebApi
		deployment                                     *appsv1.Deployment
		ready, progressing, degraded                   metav1.ConditionStatus
		readyReason, progressingReason, degradedReason string
	}{
		{
			name: "rollout complete", webApi: newWebApi(3, true), deployment: deploymentWithStatus(3, 3, 3, 3),
			ready: metav1.ConditionTrue, progressing: metav1.ConditionFalse, degraded: metav1.ConditionFalse,
			readyReason: reasonAllReplicasReady, progressingReason: reasonRolloutComplete, degradedReason: reasonNoIssues,
		},
		{
			name: "rollout in flight", webApi: newWebApi(3, true), deployment: deploymentWithStatus(4, 2, 3, 3),
			ready: metav1.ConditionFalse, progressing: metav1.ConditionTrue, degraded: metav1.ConditionFalse,
			readyReason: reasonRolloutInProgress, progressingReason: reasonRolloutInProgress, degradedReason: reasonNoIssues,
		},
		{
			name: "stale generation is not ready", webApi: newWebApi(3, true), deployment: staleGeneration,
			ready: metav1.ConditionFalse, progressing: metav1.ConditionTrue, degraded: metav1.ConditionFalse,
			readyReason: reasonRolloutInProgress, progressingReason: reasonRolloutInProgress, degradedReason: reasonNoIssues,
		},
		{
			name: "progress deadline exceeded", webApi: newWebApi(3, true), deployment: deadlineExceeded,
			ready: metav1.ConditionFalse, progressing: metav1.ConditionFalse, degraded: metav1.ConditionTrue,
			readyReason: reasonProgressDeadlineMissed, progressingReason: reasonProgressDeadlineMissed, degradedReason: reasonProgressDeadlineMissed,
		},
		{
			name: "replica failure", webApi: newWebApi(3, true), deployment: replicaFailure,
			ready: metav1.ConditionFalse, progressing: metav1.ConditionTrue, degraded: metav1.ConditionTrue,
			readyReason: reasonRolloutInProgress, progressingReason: reasonRolloutInProgress, degradedReason: reasonReplicaFailure,
		},
		{
			name: "scaled to zero", webApi: newWebApi(0, true), deployment: deploymentWithStatus(0, 0, 0, 0),
			ready: metav1.ConditionFalse, progressing: metav1.ConditionFalse, degraded: metav1.ConditionFalse,
			readyReason: reasonScaledToZero, progressingReason: reasonRolloutComplete, degradedReason: reasonNoIssues,
		},
		{
			name: "available but not all ready", webApi: newWebApi(3, true), deployment: deploymentWithStatus(3, 3, 3, 2),
			ready: metav1.ConditionFalse, progressing: metav1.ConditionFalse, degraded: metav1.ConditionFalse,
			readyReason: reasonReplicasNotReady, progressingReason: reasonRolloutComplete, degradedReason: reasonNoIssues,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conds := buildConditions(tc.webApi, tc.deployment)

			check := func(condType string, wantStatus metav1.ConditionStatus, wantReason string) {
				t.Helper()
				c := meta.FindStatusCondition(conds, condType)
				if c == nil {
					t.Fatalf("condition %s missing", condType)
				}
				if c.Status != wantStatus || c.Reason != wantReason {
					t.Errorf("%s: want %s/%s, got %s/%s (%s)", condType, wantStatus, wantReason, c.Status, c.Reason, c.Message)
				}
				if c.Message == "" {
					t.Errorf("%s: message must not be empty", condType)
				}
			}

			check(conditionReady, tc.ready, tc.readyReason)
			check(conditionProgressing, tc.progressing, tc.progressingReason)
			check(conditionDegraded, tc.degraded, tc.degradedReason)
		})
	}
}
