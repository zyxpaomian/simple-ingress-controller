package route
import (
//	"crypto/tls"
	"net/url"
//	"regexp"
	"fmt"
)

// 对应ingress rules里的规则，一个host 会匹配多个path
type RoutingTable struct {
	// CertByHost *tls.Certificate
	Backends map[string][]routingTableBackend
}

// 初始化一个新的路由表
func NewRoutingTable() *RoutingTable {
	rt := &RoutingTable{
		//certificatesByHost: make(map[string]map[string]*tls.Certificate),
		Backends: make(map[string][]routingTableBackend),
	}
	rtb, _ := newroutingTableBackend("hello", "127.0.0.1", 12345)
	rt.Backends["www.zyx.com"] = append(rt.Backends["www.zyx.com"], rtb)

	//rt.init(payload)
	return rt
}

// 根据访问的host 以及 path 获取真实的backend地址
func (rt *RoutingTable) GetBackend(host, path string) (*url.URL, error) {
	backends := rt.Backends[host]
	for _, backend := range backends {
		if backend.matches(path) {
			return backend.svcUrl, nil
		}
	}
	return nil, fmt.Errorf("no backend server found")
}