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

// MeshServiceSpec defines the desired state of MeshService
type MeshServiceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ServiceName is the name of the Kubernetes Service
	ServiceName string `json:"serviceName"`

	// Namespace where the service is deployed
	Namespace string `json:"namespace"`

	// Ports defines the service ports
	Ports []ServicePort `json:"ports"`

	// Hosts for the VirtualService
	Hosts []string `json:"hosts"`

	// Gateway reference
	Gateway GatewayReference `json:"gateway"`

	// Traffic policy settings
	TrafficPolicy *TrafficPolicy `json:"trafficPolicy,omitempty"`

	// Subsets for destination rules
	Subsets []Subset `json:"subsets,omitempty"`
}

type ServicePort struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort"`
	Protocol   string `json:"protocol"`
}

type GatewayReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type TrafficPolicy struct {
	LoadBalancer *LoadBalancerSettings `json:"loadBalancer,omitempty"`
}

type LoadBalancerSettings struct {
	Simple string `json:"simple,omitempty"` // ROUND_ROBIN, LEAST_CONN, etc.
}

type Subset struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Weight int32             `json:"weight,omitempty"`
}

// MeshServiceStatus defines the observed state of MeshService
type MeshServiceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Consistency state of the mesh resources
	ConsistencyState ConsistencyState `json:"consistencyState"`

	// Last time the consistency check was performed
	LastChecked metav1.Time `json:"lastChecked,omitempty"`

	// Violations found during consistency check
	Violations []ConstraintViolation `json:"violations,omitempty"`

	// Repair actions performed or pending
	RepairActions []RepairAction `json:"repairActions,omitempty"`

	// Related resources managed by this operator
	ManagedResources []ManagedResource `json:"managedResources,omitempty"`
}

type ConsistencyState string

const (
	Consistent    ConsistencyState = "Consistent"
	Inconsistent  ConsistencyState = "Inconsistent"
	RepairPending ConsistencyState = "RepairPending"
	RepairFailed  ConsistencyState = "RepairFailed"
	Checking      ConsistencyState = "Checking"
)

type ConstraintViolation struct {
	Type     string `json:"type"`
	Resource string `json:"resource"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // Error, Warning
}

type RepairAction struct {
	Type     string `json:"type"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

type ManagedResource struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   string `json:"version,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MeshService is the Schema for the meshservices API.
type MeshService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeshServiceSpec   `json:"spec,omitempty"`
	Status MeshServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MeshServiceList contains a list of MeshService.
type MeshServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeshService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MeshService{}, &MeshServiceList{})
}
