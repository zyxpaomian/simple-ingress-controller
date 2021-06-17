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
	rt := &RoutingTable{
		//certificatesByHost: make(map[string]map[string]*tls.Certificate),
		Backends: make(map[string][]routingTableBackend),
		Lock: &sync.RWMutex{},
	}
	// 第一次加载数据
	rt.Update(payload)
	
	return rt
}

func (rt *RoutingTable) Update(payload *watcher.Payload) {
	if payload == nil {
		return
	} 
	for _, ingressPayload := range payload.Ingresses {
		rtb, _ := newroutingTableBackend(ingressPayload.Path, ingressPayload.SvcName, ingressPayload.SvcPort)
		rt.Backends[ingressPayload.Host] = append(rt.Backends[ingressPayload.Host], rtb)
	}
}


// 根据访问的host 以及 path 获取真实的backend地址
func (rt *RoutingTable) GetBackend(host, path string) (*url.URL, error) {
	backends := rt.Backends[host]
	for _, backend := range backends {
		klog.Infof("[ingress] 主机: %v 有以下upstream: %v", host, backends)
		if backend.matches(path) {
			return backend.svcUrl, nil
		}
	}
	return nil, fmt.Errorf("no backend server found")
}
