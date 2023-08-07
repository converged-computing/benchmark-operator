/*
Copyright 2023 Lawrence Livermore National Security, LLC
 (c.f. AUTHORS, NOTICE.LLNS, COPYING)

 SPDX-License-Identifier: MIT
*/

package controllers

import (
	"context"
	"fmt"

	api "github.com/converged-computing/metrics-operator/api/v1alpha1"
	mctrl "github.com/converged-computing/metrics-operator/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ensureConfigMap ensures we've generated the read only entrypoints
func (r *MetricSetReconciler) ensureConfigMaps(
	ctx context.Context,
	set *api.MetricSet,
	sets *map[string]mctrl.MetricSet,
) (*corev1.ConfigMap, ctrl.Result, error) {

	// Look for the config map by name
	existing := &corev1.ConfigMap{}
	err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      set.Name,
			Namespace: set.Namespace,
		},
		existing,
	)

	if err != nil {

		r.Log.Info("ConfigMaps", "Status", "Not found and creating")

		// Prepare lookup of entrypoints, one per application/storage,
		// or possible multiple for a standalone metric
		data := map[string]string{}
		count := 0
		for _, s := range *sets {
			for _, es := range s.EntrypointScripts(set) {
				key := es.Name
				if key == "" {
					key = fmt.Sprintf("entrypoint-%d", count)
				}
				data[key] = es.Script
			}
			count += 1
		}
		cm, result, err := r.getConfigMap(ctx, set, data)
		if err != nil {
			r.Log.Error(
				err, "🟥️ Failed to get config map",
				"Namespace", cm.Namespace,
				"Name", (*cm).Name,
			)
		}
		return existing, result, err

	} else {
		r.Log.Info(
			"🎉 Found existing MetricSet ConfigMap",
			"Namespace", existing.Namespace,
			"Name", existing.Name,
		)
	}
	return existing, ctrl.Result{}, err
}

// getConfigMap generates the config map, when does not exist
func (r *MetricSetReconciler) getConfigMap(
	ctx context.Context,
	set *api.MetricSet,
	data map[string]string,
) (*corev1.ConfigMap, ctrl.Result, error) {

	// Create the config map with respective data!
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      set.Name,
			Namespace: set.Namespace,
		},
		Data: data,
	}
	// Finally create the config map
	r.Log.Info(
		"✨ Creating MetricSet ConfigMap ✨",
		"Namespace", cm.Namespace,
		"Name", cm.Name,
	)
	// Show data in the logs for debugging
	fmt.Println(cm.Data)

	// Actually create it
	ctrl.SetControllerReference(set, cm, r.Scheme)
	err := r.Create(ctx, cm)
	if err != nil {
		r.Log.Error(
			err, "🟥️ Failed to create MetricSet ConfigMap",
			"Namespace", cm.Namespace,
			"Name", (*cm).Name,
		)
	}
	return cm, ctrl.Result{}, err
}