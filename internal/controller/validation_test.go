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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1alpha1 "github.com/AaronStahley/webapi-operator/api/v1alpha1"
)

// These run against the envtest API server, so the CEL rules in the generated CRD are enforced.
var _ = Describe("WebApi validation", func() {
	ctx := context.Background()

	newSpec := func(name string) *apiv1alpha1.WebApi {
		return &apiv1alpha1.WebApi{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: apiv1alpha1.WebApiSpec{
				Image: apiv1alpha1.ImageSpec{Repository: "ghcr.io/stefanprodan/podinfo", Tag: "6.15.0"},
			},
		}
	}

	It("rejects the latest tag", func() {
		w := newSpec("cel-latest")
		w.Spec.Image.Tag = "latest"
		err := k8sClient.Create(ctx, w)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("the 'latest' tag is not allowed"))
	})

	It("rejects an enabled probe without a containerPort", func() {
		w := newSpec("cel-probe-no-port")
		w.Spec.HealthCheck = &apiv1alpha1.HealthCheckSpec{
			Readiness: &apiv1alpha1.ProbeSpec{Enabled: true, Path: "/readyz"},
		}
		err := k8sClient.Create(ctx, w)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("containerPort is required when a probe is enabled"))
	})

	It("accepts an enabled probe with a containerPort", func() {
		w := newSpec("cel-probe-with-port")
		w.Spec.ContainerPort = 9898
		w.Spec.HealthCheck = &apiv1alpha1.HealthCheckSpec{
			Liveness:  &apiv1alpha1.ProbeSpec{Enabled: true, Path: "/healthz"},
			Readiness: &apiv1alpha1.ProbeSpec{Enabled: true, Path: "/readyz"},
		}
		Expect(k8sClient.Create(ctx, w)).To(Succeed())
		Expect(k8sClient.Delete(ctx, w)).To(Succeed())
	})

	It("accepts disabled probes without a containerPort", func() {
		w := newSpec("cel-probe-disabled")
		w.Spec.HealthCheck = &apiv1alpha1.HealthCheckSpec{
			Liveness: &apiv1alpha1.ProbeSpec{Enabled: false},
		}
		Expect(k8sClient.Create(ctx, w)).To(Succeed())
		Expect(k8sClient.Delete(ctx, w)).To(Succeed())
	})
})
