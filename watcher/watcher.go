package watcher

import (
	"context"
	"crypto/tls"
	"sync"
	"time"
	"github.com/bep/debounce"
	"k8s.io/klog"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// 整体的payload，用于将证书和ingress列表做关联
type Payload struct {
	Ingresses       []IngressPayload
	TLSCertificates map[string]*tls.Certificate
}

// ingress payload, 记录了ingress本体以及他映射的端口
type IngressPayload struct {
	Ingress      *extensionsv1beta1.Ingress
	ServicePorts map[string]map[string]int
}

// watcher的struct, 包含了客户端以及onChange函数，用于监听变化
type Watcher struct {
	client   kubernetes.Interface
	onChange func(*Payload)
}

// 创建一个新的watcher
func New(client kubernetes.Interface, onChange func(*Payload)) *Watcher {
	return &Watcher{
		client:   client,
		onChange: onChange,
	}
}

// 运行watcher
func (w *Watcher) Run(ctx context.Context) error {
	// 每分钟去list一下
	factory := informers.NewSharedInformerFactory(w.client, time.Minute)
	secretLister := factory.Core().V1().Secrets().Lister()
	serviceLister := factory.Core().V1().Services().Lister()
	ingressLister := factory.Extensions().V1beta1().Ingresses().Lister()


	// 初次加载，加载所有的service 对象，只查询该ingress所在的namespace的service
	// 测试使用1.20的k8s, 通过ingressbackend 无法取值， 后续验证下其他版本
	// addBackend := func(ingressPayload *IngressPayload, backend extensionsv1beta1.IngressBackend) {
	addBackend := func(ingressPayload *IngressPayload, backend extensionsv1beta1.IngressBackend) {
		// 通过 Ingress 所在的 namespace 和 ServiceName 获取 Service 对象
		svc, err := serviceLister.Services(ingressPayload.Ingress.Namespace).Get(backend.ServiceName)
		if err != nil {
			klog.Errorf("[ingress] get service list failed")
		} else {
			klog.Infof("[ingress] 当前namespace下有以下service: %v", svc)
			// Service 端口映射
			m := make(map[string]int)
			for _, port := range svc.Spec.Ports {
				m[port.Name] = int(port.Port)
			}
			ingressPayload.ServicePorts[svc.Name] = m
			// {whoami: {httpport: 80, httpsport: 443}}
		}
	}

	onChange := func() {
		payload := &Payload{
			TLSCertificates: make(map[string]*tls.Certificate),
		}

		// 获得所有的 Ingress
		ingresses, err := ingressLister.List(labels.Everything())
		if err != nil {
			klog.Errorf("[ingress] failed to list ingresses")
			return
		}
		//klog.Infof("ingress list : %v", ingresses)

		for _, ingress := range ingresses {
			// 构造 IngressPayload 结构
			klog.Infof("Ingress is : %v", ingress)
			klog.Infof("Ingress Backend is : %v", ingress.Spec.Backend)
			klog.Infof("Ingress Type is : %T", ingress)
			klog.Infof("Ingress spec is : %v", ingress.Spec)
			klog.Infof("Ingress rules is : %v", ingress.Spec.Rules)
			klog.Infof("Ingress RuleValue is : %v", ingress.Spec.Rules[0])


			ingressPayload := IngressPayload{
				Ingress:      ingress,
				ServicePorts: make(map[string]map[string]int),
			}
			payload.Ingresses = append(payload.Ingresses, ingressPayload)

			//apiVersion: extensions/v1beta1
			//kind: Ingress
			//metadata:
			//  name: test-ingress
			//spec:
			//  backend:
			//    serviceName: testsvc
			//    servicePort: 80
			if len(ingress.Spec.Rules) != 0 {
				// 给 ingressPayload 组装数据
				klog.Infof("准备开始组装数据")
				for _, i := range ingress.Spec.Rules{
					klog.Infof("Ingress host is :%v", i.Host)
					klog.Infof("Ingress path is :%v", i)
					for _, j := range i.IngressRuleValue.HTTP.Paths{
						addBackend(&ingressPayload, j.Backend)
					}
					//addBackend(&ingressPayload, *ingress.Spec.Backend)
				}
				
			}
			//apiVersion: extensions/v1beta1
			//kind: Ingress
			//metadata:
			//  name: test
			//spec:
			//  rules:
			//  - host: foo.bar.com
			//    http:
			//      paths:
			//      - backend:
			//          serviceName: s1
			//          servicePort: 80
			for _, rule := range ingress.Spec.Rules {
				if rule.HTTP != nil {
					continue
				}
				for _, path := range rule.HTTP.Paths {
					// 给 ingressPayload 组装数据
					addBackend(&ingressPayload, path.Backend)
				}
			}

			// 证书处理
			for _, rec := range ingress.Spec.TLS {
				if rec.SecretName != "" {
					// 获取证书对应的 secret
					secret, err := secretLister.Secrets(ingress.Namespace).Get(rec.SecretName)
					if err != nil {
						klog.Errorf("[ingress] 获取secret 失败, %v", err)
						continue
					}
					// 加载证书
					cert, err := tls.X509KeyPair(secret.Data["tls.crt"], secret.Data["tls.key"])
					if err != nil {
						klog.Errorf("[ingress] 加载证书失败, %v", err)
						continue
					}

					payload.TLSCertificates[rec.SecretName] = &cert
				}
			}
		}

		w.onChange(payload)
	}

	debounced := debounce.New(time.Second)
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			debounced(onChange)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			debounced(onChange)
		},
		DeleteFunc: func(obj interface{}) {
			debounced(onChange)
		},
	}

	// 启动 Secret、Ingress、Service 的 Informer，用同一个事件处理器 handler
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		informer := factory.Core().V1().Secrets().Informer()
		informer.AddEventHandler(handler)
		informer.Run(ctx.Done())
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		informer := factory.Extensions().V1beta1().Ingresses().Informer()
		informer.AddEventHandler(handler)
		informer.Run(ctx.Done())
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		informer := factory.Core().V1().Services().Informer()
		informer.AddEventHandler(handler)
		informer.Run(ctx.Done())
		wg.Done()
	}()

	wg.Wait()
	return nil
}
