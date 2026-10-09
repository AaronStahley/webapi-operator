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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1alpha1 "github.com/AaronStahley/webapi-operator/api/v1alpha1"
)

// Condition types reported on a WebApi.
const (
	conditionReady       = "Ready"
	conditionProgressing = "Progressing"
	conditionDegraded    = "Degraded"
)

// Condition reasons.
const (
	reasonAllReplicasReady           = "AllReplicasReady"
	reasonReplicasNotReady           = "ReplicasNotReady"
	reasonScaledToZero               = "ScaledToZero"
	reasonRolloutInProgress          = "RolloutInProgress"
	reasonRolloutComplete            = "RolloutComplete"
	reasonProgressDeadlineMissed     = "ProgressDeadlineExceeded"
	reasonReplicaFailure             = "ReplicaFailure"
	reasonNoIssues                   = "NoIssues"
	deploymentReasonDeadlineExceeded = "ProgressDeadlineExceeded" // reason set by the Deployment controller
)

// desiredReplicas returns the replica count the WebApi asks for.
func desiredReplicas(webApi *apiv1alpha1.WebApi) int32 {
	if webApi.Spec.Replicas != nil {
		return *webApi.Spec.Replicas
	}
	return 1
}

func findDeploymentCondition(d *appsv1.Deployment, t appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for i := range d.Status.Conditions {
		if d.Status.Conditions[i].Type == t {
			return &d.Status.Conditions[i]
		}
	}
	return nil
}

// buildConditions derives the WebApi conditions from the observed Deployment.
//
//   - Ready: every desired replica is updated, available and ready.
//   - Progressing: true only while a rollout is in flight (unlike the Deployment's
//     own Progressing condition, which stays true after completion).
//   - Degraded: the rollout stalled past its progress deadline, or pods cannot be created.
func buildConditions(webApi *apiv1alpha1.WebApi, d *appsv1.Deployment) []metav1.Condition {
	desired := desiredReplicas(webApi)
	st := d.Status

	// The Deployment status is only meaningful once the controller has seen the latest spec.
	observed := st.ObservedGeneration >= d.Generation
	rolledOut := observed &&
		st.UpdatedReplicas == desired &&
		st.Replicas == desired &&
		st.AvailableReplicas == desired

	var degradedReason, degradedMessage string
	if c := findDeploymentCondition(d, appsv1.DeploymentProgressing); c != nil &&
		c.Status == corev1.ConditionFalse && c.Reason == deploymentReasonDeadlineExceeded {
		degradedReason, degradedMessage = reasonProgressDeadlineMissed, c.Message
	} else if c := findDeploymentCondition(d, appsv1.DeploymentReplicaFailure); c != nil &&
		c.Status == corev1.ConditionTrue {
		degradedReason, degradedMessage = reasonReplicaFailure, c.Message
	}
	deadlineMissed := degradedReason == reasonProgressDeadlineMissed

	gen := webApi.Generation
	progress := fmt.Sprintf("%d/%d replicas updated, %d/%d available",
		st.UpdatedReplicas, desired, st.AvailableReplicas, desired)

	// Ready
	ready := metav1.Condition{Type: conditionReady, ObservedGeneration: gen}
	switch {
	case deadlineMissed:
		ready.Status, ready.Reason, ready.Message = metav1.ConditionFalse, reasonProgressDeadlineMissed, degradedMessage
	case !rolledOut:
		ready.Status, ready.Reason, ready.Message = metav1.ConditionFalse, reasonRolloutInProgress, progress
	case desired == 0:
		ready.Status, ready.Reason, ready.Message = metav1.ConditionFalse, reasonScaledToZero, "Scaled to zero replicas"
	case st.ReadyReplicas == desired:
		ready.Status, ready.Reason = metav1.ConditionTrue, reasonAllReplicasReady
		ready.Message = fmt.Sprintf("%d/%d replicas ready", st.ReadyReplicas, desired)
	default:
		ready.Status, ready.Reason = metav1.ConditionFalse, reasonReplicasNotReady
		ready.Message = fmt.Sprintf("%d/%d replicas ready", st.ReadyReplicas, desired)
	}

	// Progressing
	progressing := metav1.Condition{Type: conditionProgressing, ObservedGeneration: gen}
	switch {
	case deadlineMissed:
		progressing.Status, progressing.Reason, progressing.Message =
			metav1.ConditionFalse, reasonProgressDeadlineMissed, "Rollout exceeded its progress deadline"
	case !rolledOut:
		progressing.Status, progressing.Reason, progressing.Message =
			metav1.ConditionTrue, reasonRolloutInProgress, progress
	default:
		progressing.Status, progressing.Reason, progressing.Message =
			metav1.ConditionFalse, reasonRolloutComplete, "Rollout complete"
	}

	// Degraded
	degraded := metav1.Condition{Type: conditionDegraded, ObservedGeneration: gen}
	if degradedReason != "" {
		degraded.Status, degraded.Reason, degraded.Message = metav1.ConditionTrue, degradedReason, degradedMessage
	} else {
		degraded.Status, degraded.Reason, degraded.Message = metav1.ConditionFalse, reasonNoIssues, "No rollout failures detected"
	}

	return []metav1.Condition{ready, progressing, degraded}
}
