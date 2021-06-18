package route

import (
//	"crypto/tls"
	"net/url"
	"regexp"
	"fmt"	
)
/*
// 以一个简单的ingress举例,制定结构体
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: zyx
spec:
  rules:
    - host: www.zyx.com
      http:
        paths:
          - path: /
            backend:
              serviceName: zyx
              servicePort: 80

*/

// 实际的service和path的正则的匹配关系, 支持正则表达式进行path的匹配
type routingTableBackend struct {
	pathRe *regexp.Regexp
	svcUrl    *url.URL 
}


// 初始化一个新的rtb对象
func newroutingTableBackend(path string, serviceName string, servicePort int) (routingTableBackend, error) {
	rtb := routingTableBackend{
		svcUrl: &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s:%d", serviceName, servicePort),
		},
	}
	var err error
	if path != "" {
		rtb.pathRe, err = regexp.Compile(path)
	}
	return rtb, err
}

func (rtb routingTableBackend) matches(path string) bool {
	if rtb.pathRe == nil {
		return true
	}
	/*if rtb.pathRe.MatchString(path) {
		rtb.svcUrl.Path = path
		return true
	} else {
		return false
	}*/
	return rtb.pathRe.MatchString(path)
}