/*
Copyright 2024.

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

package v1

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var podlog = logf.Log.WithName("pod-resource")

type PodValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

func NewPodValidator(c client.Client, d admission.Decoder) *PodValidator {
	return &PodValidator{Client: c, decoder: d}
}

// PodValidator admits a pod if a specific annotation exists.
func (v *PodValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	/*
		Get the pod in request

		Check if pod exist already

			no - return, since this is time pod being created

		yes -

		check if labels are changes

		if changed -

		check if any network policy contains these pod labels
	*/
	pod := &corev1.Pod{}

	err := v.decoder.Decode(req, pod)
	if err != nil {
		fmt.Println("Error decoding pod:", err)
		return admission.Errored(http.StatusBadRequest, err)
	}
	fmt.Printf("Received pod label validator request for %s\n\n", pod.GetName())

	// Get the original pod if it exists
	originalPod := &corev1.Pod{}
	if err := v.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, originalPod); err != nil {
		if !errors.IsNotFound(err) {
			fmt.Println("Error getting original pod:", err)
			return admission.Errored(http.StatusInternalServerError, err)
		}
		fmt.Println("Original pod not found, assuming this is a new pod creation")
		return admission.Allowed("New pod creation")
	}

	// Check if labels have changed
	if reflect.DeepEqual(originalPod.Labels, pod.Labels) {
		fmt.Println("No label changes detected for pod:", pod.GetName())
		return admission.Allowed("No label changes detected")
	}

	// Fetch NetworkPolicies in the namespace
	networkPolicyList := &networkingv1.NetworkPolicyList{}
	filters := []client.ListOption{
		client.InNamespace(pod.GetNamespace()),
	}
	if err := v.Client.List(ctx, networkPolicyList, filters...); err != nil {
		fmt.Println("Error listing network policies:", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Check if any NetworkPolicy references the original pod's labels
	for _, networkPolicy := range networkPolicyList.Items {
		if matchesLabels(originalPod.Labels, networkPolicy.Spec.PodSelector.MatchLabels) {
			fmt.Printf("Labels for pod %s are referenced in a NetworkPolicy\n", pod.GetName())
			// Log a warning but allow the change
			return admission.Allowed("").WithWarnings("Pod labels are referenced in a NetworkPolicy")
		}
	}

	fmt.Printf("Pod %s labels are not referenced in any NetworkPolicy\n", pod.GetName())
	return admission.Allowed("Labels are not referenced in any NetworkPolicy")
}

func matchesLabels(podslabels map[string]string, netPolLabels map[string]string) bool {
	selector := labels.Set(netPolLabels).AsSelectorPreValidated()
	return selector.Matches(labels.Set(podslabels))
}

// PodValidator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (v *PodValidator) InjectDecoder(d admission.Decoder) error {
	fmt.Println("Inject Decoder is called")
	v.decoder = d
	return nil
}

// func (r *PodValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
// 	return ctrl.NewWebhookManagedBy(mgr).
// 		For(r).
// 		WithValidator(&PodValidator{Client: mgr.GetClient()}).
// 		Complete()
// }
