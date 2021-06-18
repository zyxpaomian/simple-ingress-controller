package server

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"simple-ingress-controller/server/route"
	"simple-ingress-controller/watcher"
	"context"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog"
)

// server 结构体
type Server struct {
	port	int
	tlsPort	int
	routingTables *route.RoutingTable
	ready         *Event
}

// New 创建一个新的webserver
func NewServer(port, tlsPort int) *Server {
	// 需要等待watcher 第一次返回payload，所以第一后端路由表的初始化为空
	rtb := route.NewRoutingTable(nil)

	s := &Server{
		port:	port,
		tlsPort: tlsPort,
		routingTables: rtb,
		ready:         NewEvent(),
	}
	return s
}

// 开启webserver 
func (s *Server) Run(ctx context.Context) error {
	// 直到收到第一个 payload 数据后才开始监听
	s.ready.Wait(ctx)

	var eg errgroup.Group
	// 启动http server
	eg.Go(func() error {
		srv := http.Server{
			Addr:    fmt.Sprintf(":%d", s.port),
			Handler: s,
		}
		klog.Infof("[ingress] start http proxy server...")
		err := srv.ListenAndServe()
		if err != nil {
			return fmt.Errorf("start http proxy server failed, error: %v", err)
		}
		return nil
	})
	// 启动https server
	eg.Go(func() error {
		srv := http.Server{
			Addr:    fmt.Sprintf(":%d", s.tlsPort),
			Handler: s,
		}
		srv.TLSConfig = &tls.Config{
			GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				return s.routingTables.GetCertificate(hello.ServerName)
			},
		}
		klog.Infof("[ingress] start https proxy server...")
		err := srv.ListenAndServeTLS("", "")
		if err != nil {
			return fmt.Errorf("start https proxy server failed, error: %v", err)
		}
		return nil
	})
	return eg.Wait()
}

// 实际的代理服务功能
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 获取后端的真实服务地址
	klog.Infof("[ingress] get request for %s%s", r.Host, r.URL.Path)
	backendURL, err := s.routingTables.GetBackend(r.Host, r.URL.Path)
	if err != nil {
		http.Error(w, "upstream server not found", http.StatusNotFound)
		return
	}
	klog.Infof("[ingress] ready to forward backend url: %v", backendURL)
	// 使用 NewSingleHostReverseProxy 进行代理请求
	p := httputil.NewSingleHostReverseProxy(backendURL)
	p.ServeHTTP(w, r)
}

// Update 更新路由表根据新的 Ingress 规则
func (s *Server) Update(payload *watcher.Payload) {
	s.routingTables = route.NewRoutingTable(payload)
	s.ready.Set()
}

