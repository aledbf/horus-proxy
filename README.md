# horus-proxy

- Deploy echoheaders server running `kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/master/docs/examples/http-svc.yaml`
- Deploy proxy `kubectl apply -g https://raw.githubusercontent.com/aledbf/horus-proxy/master/deployment.yaml`
- Watch proxy log
- Scale up/down
- Check last request metric /last-request
