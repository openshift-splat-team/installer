// Code generated by azure-service-operator-codegen. DO NOT EDIT.
// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package storage

import (
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/configmaps"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/core"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/secrets"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +kubebuilder:rbac:groups=network.azure.com,resources=privatednszones,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.azure.com,resources={privatednszones/status,privatednszones/finalizers},verbs=get;update;patch

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Severity",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].severity"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"
// Storage version of v1api20240601.PrivateDnsZone
// Generator information:
// - Generated from: /privatedns/resource-manager/Microsoft.Network/stable/2024-06-01/privatedns.json
// - ARM URI: /subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Network/privateDnsZones/{privateZoneName}
type PrivateDnsZone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PrivateDnsZone_Spec   `json:"spec,omitempty"`
	Status            PrivateDnsZone_STATUS `json:"status,omitempty"`
}

var _ conditions.Conditioner = &PrivateDnsZone{}

// GetConditions returns the conditions of the resource
func (zone *PrivateDnsZone) GetConditions() conditions.Conditions {
	return zone.Status.Conditions
}

// SetConditions sets the conditions on the resource status
func (zone *PrivateDnsZone) SetConditions(conditions conditions.Conditions) {
	zone.Status.Conditions = conditions
}

var _ configmaps.Exporter = &PrivateDnsZone{}

// ConfigMapDestinationExpressions returns the Spec.OperatorSpec.ConfigMapExpressions property
func (zone *PrivateDnsZone) ConfigMapDestinationExpressions() []*core.DestinationExpression {
	if zone.Spec.OperatorSpec == nil {
		return nil
	}
	return zone.Spec.OperatorSpec.ConfigMapExpressions
}

var _ secrets.Exporter = &PrivateDnsZone{}

// SecretDestinationExpressions returns the Spec.OperatorSpec.SecretExpressions property
func (zone *PrivateDnsZone) SecretDestinationExpressions() []*core.DestinationExpression {
	if zone.Spec.OperatorSpec == nil {
		return nil
	}
	return zone.Spec.OperatorSpec.SecretExpressions
}

var _ genruntime.KubernetesResource = &PrivateDnsZone{}

// AzureName returns the Azure name of the resource
func (zone *PrivateDnsZone) AzureName() string {
	return zone.Spec.AzureName
}

// GetAPIVersion returns the ARM API version of the resource. This is always "2024-06-01"
func (zone PrivateDnsZone) GetAPIVersion() string {
	return "2024-06-01"
}

// GetResourceScope returns the scope of the resource
func (zone *PrivateDnsZone) GetResourceScope() genruntime.ResourceScope {
	return genruntime.ResourceScopeResourceGroup
}

// GetSpec returns the specification of this resource
func (zone *PrivateDnsZone) GetSpec() genruntime.ConvertibleSpec {
	return &zone.Spec
}

// GetStatus returns the status of this resource
func (zone *PrivateDnsZone) GetStatus() genruntime.ConvertibleStatus {
	return &zone.Status
}

// GetSupportedOperations returns the operations supported by the resource
func (zone *PrivateDnsZone) GetSupportedOperations() []genruntime.ResourceOperation {
	return []genruntime.ResourceOperation{
		genruntime.ResourceOperationDelete,
		genruntime.ResourceOperationGet,
		genruntime.ResourceOperationPut,
	}
}

// GetType returns the ARM Type of the resource. This is always "Microsoft.Network/privateDnsZones"
func (zone *PrivateDnsZone) GetType() string {
	return "Microsoft.Network/privateDnsZones"
}

// NewEmptyStatus returns a new empty (blank) status
func (zone *PrivateDnsZone) NewEmptyStatus() genruntime.ConvertibleStatus {
	return &PrivateDnsZone_STATUS{}
}

// Owner returns the ResourceReference of the owner
func (zone *PrivateDnsZone) Owner() *genruntime.ResourceReference {
	group, kind := genruntime.LookupOwnerGroupKind(zone.Spec)
	return zone.Spec.Owner.AsResourceReference(group, kind)
}

// SetStatus sets the status of this resource
func (zone *PrivateDnsZone) SetStatus(status genruntime.ConvertibleStatus) error {
	// If we have exactly the right type of status, assign it
	if st, ok := status.(*PrivateDnsZone_STATUS); ok {
		zone.Status = *st
		return nil
	}

	// Convert status to required version
	var st PrivateDnsZone_STATUS
	err := status.ConvertStatusTo(&st)
	if err != nil {
		return errors.Wrap(err, "failed to convert status")
	}

	zone.Status = st
	return nil
}

// Hub marks that this PrivateDnsZone is the hub type for conversion
func (zone *PrivateDnsZone) Hub() {}

// OriginalGVK returns a GroupValueKind for the original API version used to create the resource
func (zone *PrivateDnsZone) OriginalGVK() *schema.GroupVersionKind {
	return &schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: zone.Spec.OriginalVersion,
		Kind:    "PrivateDnsZone",
	}
}

// +kubebuilder:object:root=true
// Storage version of v1api20240601.PrivateDnsZone
// Generator information:
// - Generated from: /privatedns/resource-manager/Microsoft.Network/stable/2024-06-01/privatedns.json
// - ARM URI: /subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Network/privateDnsZones/{privateZoneName}
type PrivateDnsZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PrivateDnsZone `json:"items"`
}

// Storage version of v1api20240601.APIVersion
// +kubebuilder:validation:Enum={"2024-06-01"}
type APIVersion string

const APIVersion_Value = APIVersion("2024-06-01")

// Storage version of v1api20240601.PrivateDnsZone_Spec
type PrivateDnsZone_Spec struct {
	// AzureName: The name of the resource in Azure. This is often the same as the name of the resource in Kubernetes but it
	// doesn't have to be.
	AzureName       string                      `json:"azureName,omitempty"`
	Etag            *string                     `json:"etag,omitempty"`
	Location        *string                     `json:"location,omitempty"`
	OperatorSpec    *PrivateDnsZoneOperatorSpec `json:"operatorSpec,omitempty"`
	OriginalVersion string                      `json:"originalVersion,omitempty"`

	// +kubebuilder:validation:Required
	// Owner: The owner of the resource. The owner controls where the resource goes when it is deployed. The owner also
	// controls the resources lifecycle. When the owner is deleted the resource will also be deleted. Owner is expected to be a
	// reference to a resources.azure.com/ResourceGroup resource
	Owner       *genruntime.KnownResourceReference `group:"resources.azure.com" json:"owner,omitempty" kind:"ResourceGroup"`
	PropertyBag genruntime.PropertyBag             `json:"$propertyBag,omitempty"`
	Tags        map[string]string                  `json:"tags,omitempty"`
}

var _ genruntime.ConvertibleSpec = &PrivateDnsZone_Spec{}

// ConvertSpecFrom populates our PrivateDnsZone_Spec from the provided source
func (zone *PrivateDnsZone_Spec) ConvertSpecFrom(source genruntime.ConvertibleSpec) error {
	if source == zone {
		return errors.New("attempted conversion between unrelated implementations of github.com/Azure/azure-service-operator/v2/pkg/genruntime/ConvertibleSpec")
	}

	return source.ConvertSpecTo(zone)
}

// ConvertSpecTo populates the provided destination from our PrivateDnsZone_Spec
func (zone *PrivateDnsZone_Spec) ConvertSpecTo(destination genruntime.ConvertibleSpec) error {
	if destination == zone {
		return errors.New("attempted conversion between unrelated implementations of github.com/Azure/azure-service-operator/v2/pkg/genruntime/ConvertibleSpec")
	}

	return destination.ConvertSpecFrom(zone)
}

// Storage version of v1api20240601.PrivateDnsZone_STATUS
type PrivateDnsZone_STATUS struct {
	Conditions                                     []conditions.Condition `json:"conditions,omitempty"`
	Etag                                           *string                `json:"etag,omitempty"`
	Id                                             *string                `json:"id,omitempty"`
	InternalId                                     *string                `json:"internalId,omitempty"`
	Location                                       *string                `json:"location,omitempty"`
	MaxNumberOfRecordSets                          *int                   `json:"maxNumberOfRecordSets,omitempty"`
	MaxNumberOfVirtualNetworkLinks                 *int                   `json:"maxNumberOfVirtualNetworkLinks,omitempty"`
	MaxNumberOfVirtualNetworkLinksWithRegistration *int                   `json:"maxNumberOfVirtualNetworkLinksWithRegistration,omitempty"`
	Name                                           *string                `json:"name,omitempty"`
	NumberOfRecordSets                             *int                   `json:"numberOfRecordSets,omitempty"`
	NumberOfVirtualNetworkLinks                    *int                   `json:"numberOfVirtualNetworkLinks,omitempty"`
	NumberOfVirtualNetworkLinksWithRegistration    *int                   `json:"numberOfVirtualNetworkLinksWithRegistration,omitempty"`
	PropertyBag                                    genruntime.PropertyBag `json:"$propertyBag,omitempty"`
	ProvisioningState                              *string                `json:"provisioningState,omitempty"`
	Tags                                           map[string]string      `json:"tags,omitempty"`
	Type                                           *string                `json:"type,omitempty"`
}

var _ genruntime.ConvertibleStatus = &PrivateDnsZone_STATUS{}

// ConvertStatusFrom populates our PrivateDnsZone_STATUS from the provided source
func (zone *PrivateDnsZone_STATUS) ConvertStatusFrom(source genruntime.ConvertibleStatus) error {
	if source == zone {
		return errors.New("attempted conversion between unrelated implementations of github.com/Azure/azure-service-operator/v2/pkg/genruntime/ConvertibleStatus")
	}

	return source.ConvertStatusTo(zone)
}

// ConvertStatusTo populates the provided destination from our PrivateDnsZone_STATUS
func (zone *PrivateDnsZone_STATUS) ConvertStatusTo(destination genruntime.ConvertibleStatus) error {
	if destination == zone {
		return errors.New("attempted conversion between unrelated implementations of github.com/Azure/azure-service-operator/v2/pkg/genruntime/ConvertibleStatus")
	}

	return destination.ConvertStatusFrom(zone)
}

// Storage version of v1api20240601.PrivateDnsZoneOperatorSpec
// Details for configuring operator behavior. Fields in this struct are interpreted by the operator directly rather than being passed to Azure
type PrivateDnsZoneOperatorSpec struct {
	ConfigMapExpressions []*core.DestinationExpression `json:"configMapExpressions,omitempty"`
	PropertyBag          genruntime.PropertyBag        `json:"$propertyBag,omitempty"`
	SecretExpressions    []*core.DestinationExpression `json:"secretExpressions,omitempty"`
}

func init() {
	SchemeBuilder.Register(&PrivateDnsZone{}, &PrivateDnsZoneList{})
}
