package pod_ranker

import v1 "k8s.io/api/core/v1"

// PodRanker is an interface for filtering pods
type PodRanker interface {
	RankPods(pods []*v1.Pod) []*v1.Pod
}
