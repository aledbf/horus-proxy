# Horus Proxy - A general purpose, Scale to Zero component for Kubernetes

Horus enables greater resource efficiency within a Kubernetes cluster by
allowing idling workloads to automatically scale-to-zero and allowing
scaled-to-zero workloads to be automatically re-activated on-demand
by inbound requests.

This project started after using [Osiris](https://github.com/deislabs/osiris) ([Horus is Osiris son](https://en.wikipedia.org/wiki/Horus))

## How it works

In Kubernetes, there is no direct relationship between deployments and services.
Deployments manage pods and services may select pods managed by one or more deployments.
For this reason, the next example allows us to define this relationship and some context 
about the idle time after we want to scale to zero and the minimum number of replicas 
that should be started when we scale from zero.

Using a [Kubernetes Custom Resources or CRD](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/), we define the relationship between the deployment and the service to use plus other details about the behavior we expect.

```yaml
apiVersion: autoscaler.rocket-science.io/v1beta1
kind: Traffic
metadata:
  name: proxy-echoheaders
spec:
  deployment: echoheaders
  service: echoheaders-svc
  idleAfter: 30s
  minReplicas: 2
```

This example will use a deployment called `echoheaders` in the default namespace
with the service `echoheaders-svc`. Once horus detects the `Traffic` definition
it creates a new deployment `<deployment>-<svc>-horus-proxy` using the same
`matchLabels` defined in the deployment, adding a new selector `handled-by: horus-proxy`.
The horus controller also changes the service adding a new label
`handled-by: horus-proxy`.

Adding a new label we callows us to route traffic using the horus-proxy pod instead of the
ones defined in the original deployment. **This is the main difference with Osiris**

Horus Proxy consists in two components, a Go program (controller) and NGINX. The controller
role is the generation of the NGINX configuration file with information of the pods running
in the cluster and also the extraction of prometheus metrics from NGINX to know if there are
one or more requests being hold because there are no running pods and to know the last time
the proxy processed a request.
Once the proxy receives a request, NGINX checks if there is a running pod for the deployment.
In case there is no running pod, it holds the traffic until there is an available one. Every
five seconds the controller checks the metrics and if the metric `http_requests_waiting_endpoint`
is > 0 it means NGINX is waiting for a pod. If this happens the controller scales the deployment
to one replica. Once the pod is running the controller updates the NGINX configuration 
(using Lua) without restarting NGINX.

### Scaling to zero and the HPA

Horus is designed to work alongside the Horizontal Pod Autoscaler and is not meant to replace 
it -- it will scale your pods from n to 0 and from 0 to n, where n is a configurable minimum 
number of replicas (one, by default).
All other scaling decisions may be delegated to an HPA, if desired.

At some point, there will be no pending requests. When this happends and after the `idleAfter` 
time definition the controller will scale the deployment to zero.

## Setup

Prerequisites:

* A running Kubernetes cluster

### Install horus

TODO

### Example

TODO

# horus-proxy

- Deploy echoheaders server running `kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/master/docs/examples/http-svc.yaml`
- Deploy proxy `kubectl apply -f https://raw.githubusercontent.com/aledbf/horus-proxy/master/deployment.yaml`
- Watch proxy log
- Scale deployment `http-svc` up/down
- Check last request metric /last-request
