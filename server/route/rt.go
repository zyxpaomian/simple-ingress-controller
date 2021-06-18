package route

import (
	//	"crypto/tls"
	"net/url"
	//	"regexp"
	"fmt"
	"sync"
	"simple-ingress-controller/watcher"
	"k8s.io/klog"
)

// 对应ingress rules里的规则，一个host 会匹配多个path
type RoutingTable struct {
	// CertByHost *tls.Certificate
	Backends map[string][]routingTableBackend
	Lock *sync.RWMutex
}

// 初始化一个新的路由表
func NewRoutingTable(payload *watcher.Payload) *RoutingTable {
	klog.Infof("routetable payload is %v", payload)
	rt := &RoutingTable{
		//certificatesByHost: make(map[string]map[string]*tls.Certificate),
		Backends: make(map[string][]routingTableBackend),
		Lock: &sync.RWMutex{},
	}
	// 第一次加载数据
	rt.init(payload)
	
	return rt
}

func (rt *RoutingTable) init(payload *watcher.Payload) {
	if payload == nil {
		return
	}
	rt.Lock.Lock()
	for _, ingressPayload := range payload.Ingresses {
		rtb, _ := newroutingTableBackend(ingressPayload.Path, ingressPayload.SvcName, ingressPayload.SvcPort)
		rt.Backends[ingressPayload.Host] = append(rt.Backends[ingressPayload.Host], rtb)
		klog.Infof("[ingress] add ingress for host: %v info: %v", ingressPayload.Host, rtb)
	}
	rt.Lock.Unlock()
}


// 根据访问的host 以及 path 获取真实的backend地址
func (rt *RoutingTable) GetBackend(host, path string) (*url.URL, error) {
	klog.Infof("[ingress] 当前的backends : %v", rt.Backends)
	
	backends := rt.Backends[host]
	klog.Infof("[ingress] backend : %v", backends)
	for _, backend := range backends {
		klog.Infof("[ingress] 主机: %v 有以下upstream: %v", host, backends)
		if backend.matches(path) {
			return backend.svcUrl, nil
		}
	}
	return nil, fmt.Errorf("no backend server found")
}