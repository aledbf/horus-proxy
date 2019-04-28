package proxy

import (
	"fmt"

	"github.com/aledbf/horus-proxy/pkg/nginx"
	corev1 "k8s.io/api/core/v1"
)

const (
	handledByLabelName  = "handled-by"
	handledByLabelValue = "horus-proxy"
)

func kubeToNGINX(svc *corev1.Service, pods []*corev1.Pod) *nginx.Configuration {
	servers := make([]nginx.Server, 0)

	for _, service := range svc.Spec.Ports {
		upstreams := []nginx.Endpoint{}

		for _, pod := range pods {
			if !IsPodReady(pod) {
				continue
			}

			if _, ok := pod.Labels[handledByLabelName]; ok {
				continue
			}

			if len(pod.Status.PodIP) == 0 {
				continue
			}

			ups := nginx.Endpoint{
				Address: pod.Status.PodIP,
				Port:    service.TargetPort.String(),
			}

			upstreams = append(upstreams, ups)
		}

		servers = append(servers, nginx.Server{
			Name:      fmt.Sprintf("%v-%v-%v", svc.Namespace, svc.Name, service.TargetPort.String()),
			Port:      service.TargetPort.String(),
			Endpoints: upstreams,
		})
	}

	return &nginx.Configuration{
		Servers: servers,
	}
}
