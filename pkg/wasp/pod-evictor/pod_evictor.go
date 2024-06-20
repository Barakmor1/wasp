package pod_evictor

import v1 "k8s.io/api/core/v1"

// PodEvictor is an interface for evicting pods
type PodEvictor interface {
	EvictPod(pod *v1.Pod) error
}
