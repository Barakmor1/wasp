package pod_filter

import v1 "k8s.io/api/core/v1"

// PodFilter is an interface for filtering pods
type PodFilter interface {
	FilterPods(pods []*v1.Pod) []*v1.Pod
}
