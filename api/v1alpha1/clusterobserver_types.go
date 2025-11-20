/*
Copyright 2025.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterObserverSpec defines the desired state of ClusterObserver
type ClusterObserverSpec struct {
	// ClusterName is the identifier for this cluster in reports
	// +kubebuilder:validation:Required
	ClusterName string `json:"clusterName"`

	// ReportEndpoint is the HTTP URL where reports will be sent
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https?://.*`
	ReportEndpoint string `json:"reportEndpoint"`

	// ReportInterval defines how often to send reports (e.g., "30s", "1m")
	// +kubebuilder:validation:Required
	// +kubebuilder:default="30s"
	ReportInterval string `json:"reportInterval,omitempty"`
}

// ClusterObserverStatus defines the observed state of ClusterObserver.
type ClusterObserverStatus struct {
	// LastReportTime is the timestamp of the last successful report
	// +optional
	LastReportTime *metav1.Time `json:"lastReportTime,omitempty"`

	// IngressCount is the number of ingresses being observed
	// +optional
	IngressCount int `json:"ingressCount,omitempty"`

	// conditions represent the current state of the ClusterObserver resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ClusterObserver is the Schema for the clusterobservers API
type ClusterObserver struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ClusterObserver
	// +required
	Spec ClusterObserverSpec `json:"spec"`

	// status defines the observed state of ClusterObserver
	// +optional
	Status ClusterObserverStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterObserverList contains a list of ClusterObserver
type ClusterObserverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ClusterObserver `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterObserver{}, &ClusterObserverList{})
}
