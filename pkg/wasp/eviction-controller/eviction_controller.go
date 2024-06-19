package eviction_controller

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"kubevirt.io/wasp/pkg/client"
	"kubevirt.io/wasp/pkg/log"
	"time"
)

type EvictionController struct {
	nodeName     string
	waspCli      client.WaspClient
	podInformer  cache.SharedIndexInformer
	nodeInformer cache.SharedIndexInformer
	resyncPeriod time.Duration
	stop         <-chan struct{}
}

func NewEvictionController(waspCli client.WaspClient, podInformer cache.SharedIndexInformer, nodeInformer cache.SharedIndexInformer, nodeName string, stop <-chan struct{}) *EvictionController {
	ctrl := &EvictionController{
		nodeName:     nodeName,
		waspCli:      waspCli,
		resyncPeriod: metav1.Duration{Duration: 5 * time.Second}.Duration,
		podInformer:  podInformer,
		nodeInformer: nodeInformer,
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
	log.Log.Infof("implement")
}
