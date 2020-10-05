/*


Licensed under the Mozilla Public License (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.mozilla.org/en-US/MPL/2.0/

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

// SecurityPolicySpec defines the desired state of SecurityPolicy
type SecurityPolicySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	MID  string `json:"_id,omitempty"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	// OrgID is overwritten - no point setting this
	OrgID string `json:"org_id,omitempty"`
	// +kubebuilder:validation:Enum=active;draft;deny
	// State can be active, draft or deny
	// active: All keys are active and new keys can be created.
	// draft: All keys are active but no new keys can be created.
	// deny: All keys are deactivated and no keys can be created.
	State string `json:"state"`
	// Active must be set to `true` for Tyk to load the security policy into memory.
	Active bool `json:"active"`
	// IsInactive applies to the key itself. Allows enabling or disabling the policy without deleting it.
	IsInactive                    bool                        `json:"is_inactive,omitempty"`
	AccessRightsArray             []AccessDefinition          `json:"access_rights_array"`
	AccessRights                  map[string]AccessDefinition `json:"access_rights,omitempty"`
	Rate                          int64                       `json:"rate"`
	Per                           int64                       `json:"per"`
	QuotaMax                      int64                       `json:"quota_max"`
	QuotaRenewalRate              int64                       `json:"quota_renewal_rate"`
	ThrottleInterval              int64                       `json:"throttle_interval"`
	ThrottleRetryLimit            int                         `json:"throttle_retry_limit"`
	MaxQueryDepth                 int                         `json:"max_query_depth"`
	HMACEnabled                   bool                        `json:"hmac_enabled,omitempty"`
	EnableHTTPSignatureValidation bool                        `json:"enable_http_signature_validation,omitempty"`
	Tags                          []string                    `json:"tags,omitempty"`
	// KeyExpiresIn is the number of seconds till key expiry. For 1 hour is 3600. Default never expire or 0
	KeyExpiresIn int64            `json:"key_expires_in"`
	Partitions   PolicyPartitions `json:"partitions,omitempty"`
}

// from tyk/session.go
// AccessDefinition defines which versions of an API a key has access to
type AccessDefinition struct {
	// Namespace of the ApiDefinition resource to target
	Namespace string `json:"namespace"`
	// Name of the ApiDefinition resource to target
	Name string `json:"name"`

	// TODO: APIName should not really be needed, as is auto-set from the APIDefnition Resource
	APIName string `json:"api_name,omitempty"`
	// TODO: APIID should not really be needed, as is auto-set from the APIDefnition Resource
	APIID    string   `json:"api_id,omitempty"`
	Versions []string `json:"versions"`
	//RestrictedTypes []graphql.Type `json:"restricted_types"`
	Limit          APILimit     `json:"limit,omitempty"`
	AllowanceScope string       `json:"allowance_scope,omitempty"`
	AllowedURLs    []AccessSpec `json:"allowed_urls,omitempty"` // mapped string MUST be a valid regex
}

// APILimit stores quota and rate limit on ACL level (per API)
type APILimit struct {
	Rate               int64 `json:"rate"`
	Per                int64 `json:"per"`
	ThrottleInterval   int64 `json:"throttle_interval"`
	ThrottleRetryLimit int   `json:"throttle_retry_limit"`
	MaxQueryDepth      int   `json:"max_query_depth"`
	QuotaMax           int64 `json:"quota_max"`
	QuotaRenews        int64 `json:"quota_renews"`
	QuotaRemaining     int64 `json:"quota_remaining"`
	QuotaRenewalRate   int64 `json:"quota_renewal_rate"`
}

// AccessSpecs define what URLS a user has access to an what methods are enabled
type AccessSpec struct {
	URL     string   `json:"url"`
	Methods []string `json:"methods"`
}

type PolicyPartitions struct {
	Quota      bool `json:"quota"`
	RateLimit  bool `json:"rate_limit"`
	Complexity bool `json:"complexity"`
	Acl        bool `json:"acl"`
	PerAPI     bool `json:"per_api"`
}

// SecurityPolicyStatus defines the observed state of SecurityPolicy
type SecurityPolicyStatus struct {
	ID string `json:"id"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tykpolicies
// SecurityPolicy is the Schema for the securitypolicies API
type SecurityPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecurityPolicySpec   `json:"spec,omitempty"`
	Status SecurityPolicyStatus `json:"status,omitempty"`
}

// SecurityPolicyList contains a list of SecurityPolicy
// +kubebuilder:object:root=true
type SecurityPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecurityPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecurityPolicy{}, &SecurityPolicyList{})
}
