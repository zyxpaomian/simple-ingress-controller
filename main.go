package main

import (
	"context"
	"flag"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog"
	"math/rand"
	"runtime"
	"time"
	"simple-ingress-controller/server"
	"simple-ingress-controller/watcher"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	// 启动参数
	var port int
	var tlsPort int
	flag.IntVar(&port, "port", 80, "http server port.")
	flag.IntVar(&tlsPort, "tls-port", 443, "https server port")
	flag.Parse()

	// 从集群内的token和ca.crt获取 Config
	config, err := rest.InClusterConfig()
	// 由于我们要通过集群内部的 Service 进行服务的访问，所以不能在集群外部使用，所以不能使用 kubeconfig 的方式来获取 Config
	if err != nil {
		klog.Errorf("[ingress] 获取kubernetes 配置失败")
	}

	// 从 Config 中创建一个新的 Clientset
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Errorf("[ingress] 创建Kubernetes 客户端失败")
	}

	// http proxy server
	s := server.NewServer(80)

	// watcher service
	w := watcher.New(client, func(payload *watcher.Payload) {s.Update(payload)})
	// w := watcher.New(client, func(payload *watcher.Payload){klog.Infof("current payload is %v", payload)})

	// 多协程启动
	var eg errgroup.Group
	/*eg.Go(func() error {
		return s.Run(context.TODO())
	})*/
	eg.Go(func() error {
		return w.Run(context.TODO())
	})
	if err := eg.Wait(); err != nil {
		klog.Fatalf("[ingress] something is wrong: %v", err.Error())
	}

}
