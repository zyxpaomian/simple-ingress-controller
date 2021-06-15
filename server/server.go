package server

import (
	"fmt"
	"net/http"
	//"net/url"
	"net/http/httputil"
	"simple-ingress-controller/server/route"
	"simple-ingress-controller/watcher"
	//"github.com/docker/docker/api/types/backend"
	"context"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog"
)

// server 结构体
type Server struct {
	port          int
	routingTables *route.RoutingTable
	ready         *Event
}

// New 创建一个新的服务器
func NewServer(port int) *Server {
	// 先等待路由表初始化
	rtb := route.NewRoutingTable()

	s := &Server{
		port:          port,
		routingTables: rtb,
		ready:         NewEvent(),
	}
	// s.routingTable.Store(NewRoutingTable(nil))
	return s
}

// func (s *Server) Run(ctx context.Context) error {
func (s *Server) Run(ctx context.Context) error {
	// 直到第一个 payload 数据后才开始监听
	s.ready.Wait(ctx)

	var eg errgroup.Group
	// 启动http server
	eg.Go(func() error {
		srv := http.Server{
			Addr:    fmt.Sprintf(":%d", s.port),
			Handler: s,
		}
		klog.Infof("[ingress] starting http proxy server")
		err := srv.ListenAndServe()
		if err != nil {
			return fmt.Errorf("[ingress] start http proxy server failed, error: %v", err)
		}
		return nil
	})
	return eg.Wait()
}

// 实际的代理服务功能
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 获取后端的真实服务地址
	backendURL, err := s.routingTables.GetBackend(r.Host, r.URL.Path)
	if err != nil {
		http.Error(w, "upstream server not found", http.StatusNotFound)
		return
	}
	klog.Infof("backend url is %v", backendURL)
	/* backendURL, err := s.routingTable.Load().(*RoutingTable).GetBackend(r.Host, r.URL.Path)
	if err != nil {
		http.Error(w, "upstream server not found", http.StatusNotFound)
		return
	}
	*/

	//backendURL, _ := url.Parse("zyx.test.com:80")

	/*var backendURL *url.URL
	backendURL = new(url.URL)
	backendURL.Scheme = "http"
	backendURL.Host = "zyx.test.com:8080"
	backendURL.Path = "v1/api/user/getalluser"*/

	klog.Infof("[ingress] get proxy request from: %s%s", r.Host, r.URL.Path)
	// 使用 NewSingleHostReverseProxy 进行代理请求
	p := httputil.NewSingleHostReverseProxy(backendURL)
	// p.ErrorLog = stdlog.New(log.Logger, "", 0)
	p.ServeHTTP(w, r)
	// klog.Info(w)
}

// Update 更新路由表根据新的 Ingress 规则
func (s *Server) Update(payload *watcher.Payload) {
	s.routingTable.Store(NewRoutingTable(payload))
	s.ready.Set()
}
