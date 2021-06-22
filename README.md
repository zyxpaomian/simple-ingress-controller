# Simple-Ingress-Controller
___

> 简单的ingress controller，支持http and https。

### 部署方式
* 创建tls
```shell
openssl genrsa -out key.pem 2048
openssl req -new -x509 -key key.pem -out cert.pem -days 3650
```
* 创建secretmap 以及证书
```shell
kubectl create secret tls www.https.com --cert=cert.pem --key=key.pem
```

* 部署测试web以及对应的ingress 策略，该操作会生成1个deployment，1个service，以及2个ingress(一个http，一个https)

```shell
kubectl apply -f manifests/ingress-test.yaml
```

* 部署ingress-controller，该操作会启动ingress-controller服务，以及对应的rbac权限
```shell
kubectl apply -f manifests/ingress-controller.yaml
```



![](https://mypic-1253375948.cos.ap-shanghai.myqcloud.com/uPic/q0jXgb.png)

### 测试

* 把ingress-controller的IP绑定host：
```shell
192.168.49.2 www.http.com
192.168.49.2 www.https.com
```
* curl http的域名
```shell
debian:/opt/code/simple-ingress-controller# curl http://www.http.com/
welcome my web server!
debian:/opt/code/simple-ingress-controller#
```

* curl https的域名
```shell
debian:/opt/code/simple-ingress-controller# curl -k https://www.https.com/
welcome my web server!
debian:/opt/code/simple-ingress-controller#
```
