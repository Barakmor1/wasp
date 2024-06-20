/*
 * This file is part of the Wasp project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2023,Red Hat, Inc.
 *
 */
package wasp

import (
	"context"
	"flag"
	"io/ioutil"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/kubelet/eviction/api"
	"kubevirt.io/wasp/pkg/client"
	"kubevirt.io/wasp/pkg/informers"
	"kubevirt.io/wasp/pkg/log"
	eviction_controller "kubevirt.io/wasp/pkg/wasp/eviction-controller"
	"kubevirt.io/wasp/pkg/wasp/parser"
	"os"
	"strconv"
)

type WaspApp struct {
	evictionController           *eviction_controller.EvictionController
	podInformer                  cache.SharedIndexInformer
	nodeInformer                 cache.SharedIndexInformer
	ctx                          context.Context
	memoryAvailableThreshold     *api.Threshold
	cli                          client.WaspClient
	maxSwapInTrafficPerInterval  int
	maxSwapOutTrafficPerInterval int
	minTimeInterval              int
	waspNs                       string
	nodeName                     string
	fsRoot                       string
}

func Execute() {
	var err error
	flag.Parse()
	var app = WaspApp{}
	memoryAvailThreshold := os.Getenv("MEMORY_AVAILABLE_THRESHOLD")
	maxSwapInTrafficPerInterval := os.Getenv("MAX_SWAP_IN_TRAFFIC_PER_INTERVAL")
	maxSwapOutTrafficPerInterval := os.Getenv("MAX_SWAP_OUT_TRAFFIC_PER_INTERVAL")
	minTimeInterval := os.Getenv("MIN_TIME_INTERVAL")
	app.nodeName = os.Getenv("NODE_NAME")
	app.fsRoot = os.Getenv("FSROOT")
	app.memoryAvailableThreshold, _ = parser.ParseThresholdStatement(api.SignalMemoryAvailable, memoryAvailThreshold)
	app.minTimeInterval, err = strconv.Atoi(minTimeInterval)
	if err != nil {
		panic(err)
	}
	app.maxSwapInTrafficPerInterval, err = strconv.Atoi(maxSwapInTrafficPerInterval)
	if err != nil {
		panic(err)
	}
	app.maxSwapOutTrafficPerInterval, err = strconv.Atoi(maxSwapOutTrafficPerInterval)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.ctx = ctx

	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		panic(err)
	}
	app.waspNs = string(nsBytes)
	app.cli, err = client.GetWaspClient()
	if err != nil {
		panic(err)
	}
	app.podInformer = informers.GetPodInformer(app.cli)
	app.nodeInformer = informers.GetNodeInformer(app.cli)

	log.Log.Infof("MEMORY_AVAILABLE_THRESHOLD:%v "+
		"MAX_SWAP_IN_TRAFFIC_PER_INTERVAL:%v "+
		"MAX_SWAP_OUT_TRAFFIC_PER_INTERVAL:%v "+
		"INTERVAL:%v "+
		"nodeName: %v "+
		"ns: %v "+
		"fsRoot: %v",
		app.memoryAvailableThreshold,
		app.maxSwapInTrafficPerInterval,
		app.maxSwapOutTrafficPerInterval,
		app.minTimeInterval,
		app.nodeName,
		app.waspNs,
		app.fsRoot,
	)
	stop := ctx.Done()
	app.initEvictionController(stop)
	app.Run(stop)
}

func (waspapp *WaspApp) initEvictionController(stop <-chan struct{}) {
	waspapp.evictionController = eviction_controller.NewEvictionController(waspapp.cli,
		waspapp.podInformer,
		waspapp.nodeInformer,
		waspapp.nodeName,
		stop,
	)
}

func (waspapp *WaspApp) Run(stop <-chan struct{}) {
	go waspapp.podInformer.Run(stop)
	go waspapp.nodeInformer.Run(stop)

	if !cache.WaitForCacheSync(stop,
		waspapp.podInformer.HasSynced,
		waspapp.nodeInformer.HasSynced,
	) {
		klog.Warningf("failed to wait for caches to sync")
	}

	go func() {
		waspapp.evictionController.Run(context.Background())
	}()
	<-waspapp.ctx.Done()

}
