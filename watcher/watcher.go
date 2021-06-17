package watcher

import (
	"context"
	"crypto/tls"
	"github.com/bep/debounce"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"sync"
	"time"
)

// 整体的payload，用于将证书和ingress列表做关联
type Payload struct {
	Ingresses       []IngressPayload
	TLSCertificates map[string]*tls.Certificate
}

// ingress payload, 记录了ingress本体以及他映射的端口
type IngressPayload struct {
	Ingress *extensionsv1beta1.Ingress
	Host    string
	Path    string
	SvcName string
	SvcPort int
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
	// 每10分钟去list一下
	factory := informers.NewSharedInformerFactory(w.client, 10 * time.Minute)
	secretLister := factory.Core().V1().Secrets().Lister()
	serviceLister := factory.Core().V1().Services().Lister()
	ingressLister := factory.Extensions().V1beta1().Ingresses().Lister()

	// 初次加载，加载所有的service 对象，只查询该ingress所在的namespace的service
	// 测试使用1.20的k8s, 通过ingressbackend 无法取值， 后续验证下其他版本
	addBackend := func(ingressPayload *IngressPayload, host string, path string, backend extensionsv1beta1.IngressBackend) {
		// 通过 Ingress 所在的 namespace 和 Backend 对应的servicename 查询对应的service
		svc, err := serviceLister.Services(ingressPayload.Ingress.Namespace).Get(backend.ServiceName)
		if err != nil {
			klog.Errorf("[ingress] 获取service 失败")
		} else {
			// 进行匹配，预防service 有 而 ingress 没有
			for _, port := range svc.Spec.Ports {
				if int(port.Port) == backend.ServicePort.IntValue() {
					ingressPayload.Host = host
					ingressPayload.Path = path
					ingressPayload.SvcName = backend.ServiceName
					ingressPayload.SvcPort = backend.ServicePort.IntValue()
					klog.Infof("[ingress] 后端service 更新完成，Host: %v, Path: %v, ServiceName: %v, ServicePort: %v", ingressPayload.Host, ingressPayload.Path, ingressPayload.SvcName, ingressPayload.SvcPort)
				}
			}
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

		for _, ingress := range ingresses {
			ingressPayload := IngressPayload{
				Ingress: ingress,
			}
			payload.Ingresses = append(payload.Ingresses, ingressPayload)

			for _, rule := range ingress.Spec.Rules {
				
				if rule.HTTP == nil {
					klog.Infof("数据为空，不组装")
					continue
				}
				klog.Infof("准备开始组装数据")
				for _, path := range rule.HTTP.Paths {
					// 给 ingressPayload 组装数据
					addBackend(&ingressPayload, rule.Host, path.Path, path.Backend)
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
