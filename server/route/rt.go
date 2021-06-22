package route

import (
	"crypto/tls"
	"net/url"
	//	"regexp"
	"strings"
	"fmt"
	"sync"
	"simple-ingress-controller/watcher"
	"k8s.io/klog"
)

// 对应ingress rules里的规则，一个host 会匹配多个path
type RoutingTable struct {
	CertificatesByHost map[string]map[string]*tls.Certificate
	Backends map[string][]routingTableBackend
	Lock *sync.RWMutex
}

// 初始化一个新的路由表
func NewRoutingTable(payload *watcher.Payload) *RoutingTable {
	rt := &RoutingTable{
		CertificatesByHost: make(map[string]map[string]*tls.Certificate),
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

	// 加个锁，避免线程安全
	rt.Lock.Lock()
	for _, ingressPayload := range payload.Ingresses {
		// 先加载证书信息
		for _, rule := range ingressPayload.Ingress.Spec.Rules{
			m, ok := rt.CertificatesByHost[rule.Host]
			if !ok {
				m = make(map[string]*tls.Certificate)
				rt.CertificatesByHost[rule.Host] = m
			}
			// 更新路由表证书信息
			for _, t := range ingressPayload.Ingress.Spec.TLS {
				for _, h := range t.Hosts {
					cert, ok := payload.TLSCertificates[t.SecretName]
					if ok {
						m[h] = cert
					}
				}
			}
		}

		// 更新路由信息
		for _, rule := range ingressPayload.Ingress.Spec.Rules{
			for _, path := range rule.HTTP.Paths {
				rtb, _ := newroutingTableBackend(path.Path, path.Backend.ServiceName, path.Backend.ServicePort.IntValue())
				rt.Backends[rule.Host] = append(rt.Backends[rule.Host], rtb)
				klog.Infof("[ingress] add ingress for host: %v servicename: %v, service port: %v", rule.Host, path.Backend.ServiceName, path.Backend.ServicePort.IntValue())
			}
		}
	}
	rt.Lock.Unlock()
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

// 判断证书是否正确
func (rt *RoutingTable) matches(sni string, certHost string) bool {
	for strings.HasPrefix(certHost, "*.") {
		if idx := strings.IndexByte(sni, '.'); idx >= 0 {
			sni = sni[idx+1:]
		} else {
			return false
		}
		certHost = certHost[2:]
	}
	return sni == certHost
}

// GetCertificate gets a certificate.
func (rt *RoutingTable) GetCertificate(sni string) (*tls.Certificate, error) {
	klog.Infof("sni is %v", sni)
	klog.Infof("rt cert is %v", rt.CertificatesByHost)
	hostCerts, ok := rt.CertificatesByHost[sni]
	if ok {
		for h, cert := range hostCerts {
			if rt.matches(sni, h) {
				return cert, nil
			}
		}
	}
	return nil, fmt.Errorf("certificate not found")
}