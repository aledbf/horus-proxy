package proxy

import (
	"fmt"

	"github.com/aledbf/horus-proxy/pkg/nginx"
	corev1 "k8s.io/api/core/v1"
)

func kubeToNGINX(svc *corev1.Service, endpoints *corev1.Endpoints) *nginx.Configuration {
	servers := make([]nginx.Server, 0)

	for _, service := range svc.Spec.Ports {
		upstreams := []nginx.Endpoint{}

		for i := range endpoints.Subsets {
			ss := &endpoints.Subsets[i]
			for i := range ss.Ports {
				epPort := &ss.Ports[i]
				if epPort.Protocol != corev1.ProtocolTCP {
					continue
				}

				var targetPort int32

				if service.Name == "" {
					// port.Name is optional if there is only one port
					targetPort = epPort.Port
				} else if service.Name == epPort.Name {
					targetPort = epPort.Port
				}

				if targetPort == 0 {
					continue
				}

				for i := range ss.Addresses {
					epAddress := &ss.Addresses[i]
					ups := nginx.Endpoint{
						Address: epAddress.IP,
						Port:    fmt.Sprintf("%v", targetPort),
					}

					upstreams = append(upstreams, ups)
				}
			}

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
