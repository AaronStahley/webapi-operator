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
	"k8s.io/apimachinery/pkg/util/intstr"

	apiv1alpha1 "github.com/AaronStahley/webapi-operator/api/v1alpha1"
)

// defaultProgressDeadlineSeconds matches the Kubernetes default. It is set
// explicitly so the desired and live Deployment never differ on this field.
const defaultProgressDeadlineSeconds int32 = 600

// rolloutSettings is the effective rollout configuration for a WebApi after
// defaults have been applied.
type rolloutSettings struct {
	maxSurge                intstr.IntOrString
	maxUnavailable          intstr.IntOrString
	minReadySeconds         int32
	progressDeadlineSeconds int32
}

// readinessProbeEnabled reports whether the WebApi enables a readiness probe.
func readinessProbeEnabled(webApi *apiv1alpha1.WebApi) bool {
	hc := webApi.Spec.HealthCheck
	return hc != nil && hc.Readiness != nil && hc.Readiness.Enabled
}

// effectiveRollout merges spec.rollout over the operator defaults.
//
// With a readiness probe the default is a zero-downtime rolling update: surge by
// one pod and never take a serving pod away before its replacement is ready.
// Without one there is nothing to gate on, so the Kubernetes default of 25% is
// kept. Kubernetes rejects a Deployment where both values are zero, so setting
// maxSurge to 0 requires a non-zero maxUnavailable.
func effectiveRollout(webApi *apiv1alpha1.WebApi) rolloutSettings {
	s := rolloutSettings{
		maxSurge:                intstr.FromString("25%"),
		maxUnavailable:          intstr.FromString("25%"),
		progressDeadlineSeconds: defaultProgressDeadlineSeconds,
	}

	if readinessProbeEnabled(webApi) {
		s.maxSurge = intstr.FromInt32(1)
		s.maxUnavailable = intstr.FromInt32(0)
	}

	if ro := webApi.Spec.Rollout; ro != nil {
		if ro.MaxSurge != nil {
			s.maxSurge = *ro.MaxSurge
		}
		if ro.MaxUnavailable != nil {
			s.maxUnavailable = *ro.MaxUnavailable
		}
		s.minReadySeconds = ro.MinReadySeconds
		if ro.ProgressDeadlineSeconds != nil {
			s.progressDeadlineSeconds = *ro.ProgressDeadlineSeconds
		}
	}

	return s
}
