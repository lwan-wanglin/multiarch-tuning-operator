---
title: Supporting Multiarch Tuning Operator on non-ocp clusters
authors:
  - "@lwan-wanglin"
reviewers:
  - "@aleskandro"
  - "@AnnaZivkovic"
  - "@jeffdyoung"
  - "@Prashanth684"
approvers:
  - "@aleskandro"
  - "@Prashanth684"
creation-date: 2025-04-022
last-updated: 2025-04-022
tracking-link:
  - https://issues.redhat.com/browse/MULTIARCH-5324
see-also: []
---

# Supporting Multiarch Tuning Operator on non-OCP clusters

## Summary
Currently, the Multiarch Tuning Operator has been successfully released on the OpenShift Container Platform (OCP).
To extend its usability, we aim to make the operator compatible with upstream Kubernetes clusters.
To run the Multiarch Tuning Operator on standard Kubernetes clusters, several OpenShift-specific limitations need to be addressed:
(a) Global Pull Secret – used to authenticate against image registries. In OCP, the global pull secret is managed by 
the `image-registry` operator and stored in the `openshift-config` namespace.
(b) `Global CA Bundle` – used for TLS handshake and verification. In OCP, the `configmap/trusted-ca` with the label 
`config.openshift.io/inject-trusted-cabundle=true` is automatically synchronized with the `spec.trustedCA` field of the 
proxy or cluster configuration.
(c) TLS Certificates – used for securing Multiarch Tuning Operator `services` and `webhook configurations`. In OCP, 
resources labeled with `service.beta.openshift.io/inject-cabundle: "true"` are automatically injected with TLS 
certificate data by the `service-ca` operator.
These resources are not automatically available in non-OCP clusters, and we should not expect users to manually create 
OpenShift-specific resources before installing the operator.
In [MULTIARCH-5324](https://issues.redhat.com/browse/MULTIARCH-5324), we propose introducing two fields in the ClusterPodPlacementConfig CRD: 
(a) `GlobalPullSecretRef` - Allows users to reference their own pull secret, enabling the controller to inspect container images.
(b) `CABundleConfigmapRef` - Allows users to reference a ConfigMap containing the CA bundle to verify registry TLS certificates. 
If not set, the operator will attempt to read CA certificates from cluster nodes and generate a default bundle automatically.
Additionally, (c) TLS Certificate Management – Automatically create and inject TLS certificates for the operator services, 
controller, and Webhook Configuration in non-OCP clusters.
(d) Test Adjustments – Skip e2e test cases that rely on OpenShift-specific resources (e.g., `Build`, `DeploymentConfig`, or 
`image.config.openshift.io`) when running in non-OCP clusters.

## Motivation
- Supporting Kubernetes outside of OCP will make the Multiarch Tuning Operator more accessible and usable in diverse environments.

### User Stories
- As a cluster administrator, I want to install and use the Multiarch Tuning Operator on a standard Kubernetes cluster without requiring OpenShift.

### Goals
- Provide a way for users to install and use the Multiarch Tuning Operator on their non-OCP Kubernetes clusters.

### Non-Goals
- The pull secret must be provided by the user; the operator will not automatically create it for non-OCP Kubernetes clusters.
- When the `CABundleConfigmapRef` field is not set, the operator will attempt to generate a default CA bundle. However, it does not guarantee that this bundle will be able to verify all registry TLS certificates.

## Proposal
We aim to make the operator compatible with standard Kubernetes clusters.

Its development is expected to ship in the following phases:
- Phase 1: Support for CRI-O Runtime-Based Kubernetes Clusters
  - Automatically generate TLS certificates for the required services and webhook configurations.
  - Provide a mechanism for users to reference their own global pull secrets.
  - Provide a mechanism for users to reference their own CA bundles.
  - Skip e2e test cases related to OpenShift-specific resources such as `Build`, `DeploymentConfig`, and `image.config.openshift.io` on non-OCP Kubernetes clusters.
- Phase 2: Adding Containerd and Docker Support
  - Extend support beyond `CRI-O` to include more runtime environments e.g. `containerd` and `Docker`.
  - Implement additional validation and compatibility checks.
  - Ensure consistent behavior across different container runtimes.

### Phase 1

#### Global pull secret for inspecting the images
Pull Secret Handling, In OpenShift, the operator retrieves pull secrets from `openshift-config/pull-secret`.
we should never expect a non-ocp user to create it manually before install our operator, for non-OCP environments,
we propose defining `GlobalPullSecretRef` as a namespaced resource inside `ClusterPodPlacementConfig` CRD,
the operator will read and apply the pull secret globally.

#### CA bundle to verify registry certification
In an OpenShift cluster, the `service-ca` operator automatically injects the CA bundle into a ConfigMap if it has the label 
`service.beta.openshift.io/inject-cabundle: "true"`.
In a standard Kubernetes cluster, we propose defining a `CABundleConfigmapRef` field in the `ClusterPodPlacementConfig` CRD, allowing users 
to reference a namespaced ConfigMap that stores the CA bundle for verifying registry TLS certificates.
If this field is not specified, the operator will attempt to retrieve the CA bundle from cluster nodes and automatically generate a default ConfigMap.

#### TLS Certificate for the operator, the controllers and the webhook configuration
TLS Certificate Management, OpenShift automatically injects TLS certificates if resources with `service.beta.openshift.io/inject-cabundle: "true"` label.
In Kubernetes, we will implement a mechanism to generate and manage certificates for the following services
  - `multiarch-tuning-operator-controller-manager-service-cert`
  - `pod-placement-controller`
  - `pod-placement-web-hook`
and webhook configurations. 
We will refer to the [Manage TLS Certificates in a Cluster](https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/) document to explore options for managing the operator certificate.

#### Add Validation for Newly Added Fields
For OpenShift clusters, users should not be allowed to configure the two newly added fields. Validation logic will be added in [clusterpodplacementconfig_webhook.go](https://github.com/openshift/multiarch-tuning-operator/blob/main/apis/multiarch/v1beta1/clusterpodplacementconfig_webhook.go) to restrict users from setting or modifying these fields when the operator is running on an OpenShift cluster.

#### ClusterPodPlacementConfig CR

```go
type ClusterPodPlacementConfigSpec struct {
  // CABundleConfigmapRef is a reference to the ConfigMap containing the CA bundle
  // used to verify registry TLS certificates.
  // +optional
  CABundleConfigmapRef *corev1.LocalObjectReference `json:"caBundleConfigmapRef,omitempty"`
  // GlobalPullSecretRef is a reference to the Secret used for authenticating image registry pulls.
  // +optional
  GlobalPullSecretRef *corev1.SecretReference `json:"globalPullSecretRef,omitempty"`
}
```

```yaml
apiVersion: multiarch.openshift.io/v1beta1
kind: ClusterPodPlacementConfig
metadata:
  name: cluster
spec:
...
  caBundleConfigmapRef:
    name: trusted-ca
  globalPullSecretRef:
    name: pull-secret
    namespace: pull-secret-ns
...
```
### Implementation Details/Notes/Constraints

### Risks and Mitigations

### Drawbacks

### Open Questions

### Test Plan

#### Unit Testing and Integration Test Suites

- Unit Testing: Test each new function, method, and feature in isolation to ensure correctness, reliability, and
  robustness. Verify that the new code paths are covered by the unit tests and that the code behaves as expected
  under different conditions.
- Integration Test Suite: Run integration tests against a simulated control plane using the operator SDK's envtest
  facilities. We will add the necessary test cases to ensure the 
  the new added field `GlobalPullSecretRef` and `CAbundleConfigmapRef` 
  is working as expected.

#### Functional Test Suite

- The operator should reject configuration of `GlobalPullSecretRef` and `CABundleConfigmapRef` on OpenShift clusters.
- The operator should not introduce any regressions on OpenShift clusters, and all e2e tests should pass.
- The operator should allow setting `GlobalPullSecretRef` on standard Kubernetes clusters, and it can be used to inspect container images.
- The operator should allow setting `CABundleConfigmapRef` on standard Kubernetes clusters, and it can be used to verify TLS connections to registries.
- If the `CABundleConfigmapRef` field is not set on non-OpenShift clusters, the operator should automatically create a CA bundle ConfigMap using certificates read from cluster nodes.
- Skip OpenShift-specific e2e test cases on non-OpenShift clusters to ensure the test suite passes.

### Graduation Criteria

### Upgrade / Downgrade Strategy
- No special upgrade/downgrade strategy is required for this enhancement. The operator will be updated to support the
kubernetes clusters based cri-o runtime

### Version Skew Strategy

### Operational Aspects of API Extensions

#### Failure Modes
- Webhook failure - The `ClusterPodPlacementConfig` validating webhook is 
  configured with "FailurePolicy=Fail". If the validation fails, the creation or update of a `ClusterPodPlacementConfig` resource will be blocked.
  The mutating admission webhook has the "FailPolicy=Ignore"
  setting. The creation or scheduling of pods will not be blocked if the webhook
  is down. However, there would be an event in the pod events log to notify the
  administrator about this condition. 
- Operator/controller failure - Any operator/controller failure will be
  localized to the operator namespace and will not affect the other,
  especially core, components. Pods might be in a gated state waiting
  to be scheduled. Once the controller/operator recovers, these pods will be
  evaluated and will proceed to be scheduled. If the operator or controller
  cannot recover, the scheduling gate has to be removed manually by patching the
  pod spec.

#### Support Procedures
- Webhook
	- If the webhook fails to deploy and run for whatever reason, alerts will
	  notify the administrator about the problem.
  - The `ClusterPodPlacementConfig` validating webhook is configured with "FailurePolicy=Fail". If the validation fails, the creation or update of a `ClusterPodPlacementConfig` resource will be blocked.
  - The mutating admission webhook has `FailPolicy=Ignore` and hence will not
  block the pod from being scheduled if any errors occur when calling the
  webhook.
  - When the webhook is online, operations will proceed as usual, and pods
  will start being intercepted and gated depending on the configuration
- Pods are gated, and the controller is down
	- If the webhook has gated certain pods and the controller unexpectedly goes
	  down, pods will be gated till it recovers
	- The scheduling gate can be manually removed from the pods to continue
	  normal operations. Pods that are gated can be identified by their status,
	  which would be "SchedulingGated" instead of "Running" or "Pending"
	- Redeploying the operator if it does not recover should start the
	  controller which would resume processing the gated pods.
	- Information about the local controllers status will be available in all the `PodPlacementConfig` of that namespace.
- Health checking on the controller will throw alerts if the controller cannot
  be reached
- Metrics can also be used to identify faulty behaviors of the controller and
  the webhook

## Documentation Plan

Provide a detailed installation guide for Kubernetes users.
Document how to configure pull secrets, and CA bundles manually if needed.

## Implementation History

## Alternatives

## Infrastructure Needed

## Open Questions
