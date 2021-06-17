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
	// 先等待路由表初始化, 第一次初始化为空
	rtb := route.NewRoutingTable(nil)

	s := &Server{
		port:          port,
		routingTables: rtb,
		ready:         NewEvent(),
	}
	return s
}

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

	klog.Infof("[ingress] get proxy request from: %s%s", r.Host, r.URL.Path)
	// 使用 NewSingleHostReverseProxy 进行代理请求
	p := httputil.NewSingleHostReverseProxy(backendURL)
	// p.ErrorLog = stdlog.New(log.Logger, "", 0)
	p.ServeHTTP(w, r)
}

// Update 更新路由表根据新的 Ingress 规则
func (s *Server) Update(payload *watcher.Payload) {
	s.routingTables.Lock.Lock()
	s.routingTables.Update(payload)
	s.routingTables.Lock.Unlock()
	s.ready.Set()
}

