/*
Copyright 2023 Red Hat, Inc.

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

package v1beta1

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-multiarch-openshift-io-v1beta1-podplacementconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=multiarch.openshift.io,resources=podplacementconfigs,verbs=create;update,versions=v1beta1,name=vpodplacementconfig.kb.io,admissionReviewVersions=v1

func (c *PodPlacementConfig) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		WithValidator(&PodPlacementConfigValidator{}).
		Complete()
}

type PodPlacementConfigValidator struct {
	client.Client
}

func (v *PodPlacementConfigValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return v.validate(ctx, obj)
}

func (v *PodPlacementConfigValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	return v.validate(ctx, newObj)
}

func (v *PodPlacementConfigValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (v *PodPlacementConfigValidator) validate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	ppcNew, ok := obj.(*PodPlacementConfig)
	if !ok {
		return nil, errors.New("not a PodPlacementConfig")
	}
	clusterPodPlacementConfig := &ClusterPodPlacementConfig{}
	err = v.Get(ctx, client.ObjectKey{
		Name: "cluster",
	}, clusterPodPlacementConfig)
	if err != nil && client.IgnoreNotFound(err) == nil {
		return nil, errors.New("Please create cluster scope ClusterPodPlacementConfig before creating namespaced PodPlacementConfig")
	}
	ppcs := &PodPlacementConfigList{}
	if err := v.List(ctx, ppcs, client.InNamespace(ppcNew.Namespace)); err != nil {
		return nil, errors.New(fmt.Sprintf("failed to list PodPlacementConfig in namespace %s: %w", ppcNew.Namespace, err))
	}
	for _, existing := range ppcs.Items {
		if existing.Name == ppcNew.Name {
			continue
		}
		if existing.Spec.Priority == ppcNew.Spec.Priority {
			return nil, errors.New(fmt.Sprintf(
				"validation denied: another PodPlacementConfig (%s) with priority %d already exists in namespace %s",
				existing.Name, existing.Spec.Priority, ppcNew.Namespace,
			))
		}
	}
	return nil, nil
}
