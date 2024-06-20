package eviction_controller

import (
	"context"
	"encoding/json"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"kubevirt.io/wasp/pkg/client"
	"kubevirt.io/wasp/pkg/log"
	pod_evictor "kubevirt.io/wasp/pkg/wasp/pod-evictor"
	pod_filter "kubevirt.io/wasp/pkg/wasp/pod-filter"
	pod_ranker "kubevirt.io/wasp/pkg/wasp/pod-ranker"
	shortage_detector "kubevirt.io/wasp/pkg/wasp/shortage-detector"
	"time"
)

const (
	timeToWaitForCacheSync = 10 * time.Second
	WaspTaint              = "waspEvictionTaint"
)

type EvictionController struct {
	shortageDetector shortage_detector.ShortageDetector
	podFilter        pod_filter.PodFilter
	podRanker        pod_ranker.PodRanker
	podEvictor       pod_evictor.PodEvictor
	nodeName         string
	waspCli          client.WaspClient
	podInformer      cache.SharedIndexInformer
	nodeInformer     cache.SharedIndexInformer
	nodeLister       v1lister.NodeLister
	resyncPeriod     time.Duration
	stop             <-chan struct{}
}

func NewEvictionController(waspCli client.WaspClient, podInformer cache.SharedIndexInformer, nodeInformer cache.SharedIndexInformer, nodeName string, stop <-chan struct{}) *EvictionController {
	ctrl := &EvictionController{
		nodeName:     nodeName,
		waspCli:      waspCli,
		resyncPeriod: metav1.Duration{Duration: 5 * time.Second}.Duration,
		podInformer:  podInformer,
		nodeInformer: nodeInformer,
		nodeLister:   v1lister.NewNodeLister(nodeInformer.GetIndexer()),
		stop:         stop,
	}
	return ctrl
}

func (ctrl *EvictionController) Run(ctx context.Context) {
	defer utilruntime.HandleCrash()
	log.Log.Infof("Starting ARQ controller")
	defer log.Log.Infof("Shutting ARQ Controller")

	go wait.Until(ctrl.HandleMemorySwapEviction, ctrl.resyncPeriod, ctrl.stop)

	<-ctx.Done()
}

func (ctrl *EvictionController) HandleMemorySwapEviction() {
	shouldEvict, err := ctrl.shortageDetector.ShouldEvict()
	if err != nil {
		log.Log.Infof(err.Error())
		return
	}
	node, err := ctrl.getNode()
	if err != nil {
		log.Log.Infof(err.Error())
		return
	}
	evicting := nodeHasEvictionTaint(node)

	switch {
	case evicting && !shouldEvict:
		err := removeWaspEvictionTaint(ctrl.waspCli, node)
		if err != nil {
			log.Log.Infof(err.Error())
			return
		}
	case !evicting && shouldEvict:
		err := addWaspEvictionTaint(ctrl.waspCli, node)
		if err != nil {
			log.Log.Infof(err.Error())
			return
		}
	}
	if !shouldEvict {
		return
	}
	pods, err := ctrl.listPodsOnNode()
	if err != nil {
		log.Log.Infof(err.Error())
		return
	}

	filteredPods := ctrl.podFilter.FilterPods(pods)
	rankedFilterdPods := ctrl.podRanker.RankPods(filteredPods)

	if len(rankedFilterdPods) == 0 {
		log.Log.Infof("Wasp evictor doesn't have any pod to evict")
		return
	}

	err = ctrl.podEvictor.EvictPod(rankedFilterdPods[0])
	if err != nil {
		log.Log.Infof(err.Error())
	}
}

func nodeHasEvictionTaint(node *v1.Node) bool {
	// Check if the node has the specified taint
	for _, taint := range node.Spec.Taints {
		if taint.Key == WaspTaint && taint.Effect == v1.TaintEffectNoSchedule {
			return true
		}
	}
	return false
}

func addWaspEvictionTaint(waspCli client.WaspClient, node *v1.Node) error {
	taint := v1.Taint{
		Key:    WaspTaint,
		Effect: v1.TaintEffectNoSchedule,
	}

	taints := append(node.Spec.Taints, taint)

	taintsPatch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"taints": taints,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal taints patch: %v", err)
	}

	_, err = waspCli.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.StrategicMergePatchType, taintsPatch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch node: %v", err)
	}

	return nil
}

func removeWaspEvictionTaint(waspCli client.WaspClient, node *v1.Node) error {
	var newTaints []v1.Taint
	for _, taint := range node.Spec.Taints {
		if taint.Key != WaspTaint {
			newTaints = append(newTaints, taint)
		}
	}

	taintsPatch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"taints": newTaints,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal taints patch: %v", err)
	}

	_, err = waspCli.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.StrategicMergePatchType, taintsPatch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch node: %v", err)
	}

	return nil
}

func waitForSyncedStore(timeout <-chan time.Time, informerSynced func() bool) bool {
	for !informerSynced() {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-timeout:
			return informerSynced()
		}
	}

	return true
}

func (ctrl *EvictionController) listPodsOnNode() ([]*v1.Pod, error) {
	if !waitForSyncedStore(time.After(timeToWaitForCacheSync), ctrl.podInformer.HasSynced) {
		log.Log.Infof("nodes caches not synchronized")
	}
	objs, err := ctrl.podInformer.GetIndexer().ByIndex("node", ctrl.nodeName)
	if err != nil {
		return nil, err
	}
	var pods []*v1.Pod
	for _, obj := range objs {
		pods = append(pods, obj.(*v1.Pod))
	}
	return pods, nil
}

func (ctrl *EvictionController) getNode() (*v1.Node, error) {
	if !waitForSyncedStore(time.After(timeToWaitForCacheSync), ctrl.nodeInformer.HasSynced) {
		log.Log.Infof("nodes caches not synchronized")
	}
	return ctrl.nodeLister.Get(ctrl.nodeName)
}
