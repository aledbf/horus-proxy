/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proxy

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	apiscore "k8s.io/kubernetes/pkg/apis/core"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/aledbf/horus-proxy/pkg/env"
	"github.com/aledbf/horus-proxy/pkg/metrics"
	"github.com/aledbf/horus-proxy/pkg/nginx"
)

var log = logf.Log.WithName("controller")

// Add creates a new Traffic Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileTraffic{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetRecorder("proxy-controller"),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Inject dependencies into Reconciler
	if err := mgr.SetFields(r); err != nil {
		return err
	}

	ngx, err := nginx.NewInstance(nginx.DefaultTemplate)
	if err != nil {
		return err
	}

	// Create a new controller
	c, err := controller.New("proxy-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	log.Info("Checking environment variables...")
	config, err := env.Parse()
	if err != nil {
		return err
	}

	kubeclient := kubernetes.NewForConfigOrDie(mgr.GetConfig())

	log.Info("Checking service and namespace...", "service", config.Service, "namespace", config.Namespace)
	_, err = kubeclient.CoreV1().Namespaces().Get(config.Namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}

	_, err = kubeclient.CoreV1().Services(config.Namespace).Get(config.Service, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = mgr.Add(manager.RunnableFunc(func(s <-chan struct{}) error {
		log.Info("Starting nginx process")
		err := ngx.Start(s)
		if err != nil {
			return err
		}

		<-s
		return nil
	}))
	if err != nil {
		return err
	}

	kubeInformerFactory := kubeinformers.NewFilteredSharedInformerFactory(kubeclient, 10*time.Minute, config.Namespace,
		func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector(apiscore.ObjectNameField, config.Service).String()
		},
	)

	// Instruct the manager to start the informers
	err = mgr.Add(manager.RunnableFunc(func(s <-chan struct{}) error {
		kubeInformerFactory.Start(s)
		<-s
		return nil
	}))
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Informer{Informer: kubeInformerFactory.Core().V1().Services().Informer()},
		&handler.EnqueueRequestForObject{},
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Informer{Informer: kubeInformerFactory.Core().V1().Endpoints().Informer()},
		&handler.EnqueueRequestForObject{},
	)
	if err != nil {
		return err
	}

	err = mgr.Add(manager.RunnableFunc(func(s <-chan struct{}) error {
		go setupScalingMonitor(config, kubeclient, s)
		<-s
		return nil
	}))
	if err != nil {
		return err
	}

	r.(*ReconcileTraffic).servicesLister = kubeInformerFactory.Core().V1().Services().Lister()
	r.(*ReconcileTraffic).endpointLister = kubeInformerFactory.Core().V1().Endpoints().Lister()
	r.(*ReconcileTraffic).Configuration = config

	r.(*ReconcileTraffic).nginx = ngx

	return nil
}

var _ reconcile.Reconciler = &ReconcileTraffic{}

// ReconcileTraffic reconciles a Traffic object
type ReconcileTraffic struct {
	Configuration *env.Spec

	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder

	nginx nginx.NGINX

	servicesLister listerscorev1.ServiceLister
	endpointLister listerscorev1.EndpointsLister
}

// Reconcile reads that state of the cluster for a Traffic object and makes changes based on the state read
// and what is in the Traffic.Spec
func (r *ReconcileTraffic) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	namespace := r.Configuration.Namespace
	service := r.Configuration.Service

	svc, err := r.servicesLister.Services(namespace).Get(service)
	if err != nil {
		return reconcile.Result{}, err
	}

	if svc.Spec.Type == corev1.ServiceTypeExternalName {
		return reconcile.Result{}, fmt.Errorf("Service type ExternalName is not supported")
	}

	endpoints, err := r.endpointLister.Endpoints(namespace).Get(service)
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(endpoints.Subsets) == 0 {
		log.Info("Service without active endpoints", "namespace", namespace, "service", service)
	}

	cfg := kubeToNGINX(svc, endpoints)

	err = r.nginx.Update(cfg)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func setupScalingMonitor(config *env.Spec, client kubernetes.Interface, stopCh <-chan struct{}) {
	collector := metrics.NewCollector()
	go collector.Start(stopCh)

	// wait before start collecting metrics
	time.Sleep(3 * time.Second)

	// check desired state interval
	t := time.NewTicker(8 * time.Second)

	idleAfter := *config.IdleAfter

	for {
		select {
		case <-t.C:
			stats := collector.CurrentStats()
			log.V(2).Info("metrics", "lastRequest", stats.LastRequest, "idleAfter", idleAfter, "endpointCount", stats.EndpointCount)

			if stats.WaitingForPods {
				log.Info("Scaling deployment up due pending requests")
				err := scaleDeployment(config.Namespace, config.Deployment, int32(1), client)
				if err != nil {
					log.Error(err, "scaling deployment to 1 replica")
				}

				continue
			}

			if stats.EndpointCount == 0 {
				// avoid access to apiserver running unnecessary scaling action
				continue
			}

			if stats.LastRequest >= int(idleAfter.Seconds()) {
				log.Info("Scaling deployment to zero due inactivity", "after", idleAfter)
				err := scaleDeployment(config.Namespace, config.Deployment, int32(0), client)
				if err != nil {
					log.Error(err, "scaling deployment to 0 replicas")
				}
			}
		case <-stopCh:
			log.Info("stoping collection of metrics")
			break
		}
	}
}

func scaleDeployment(namespace, name string, replicas int32, client kubernetes.Interface) error {
	deployment, err := client.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment.Spec.Replicas = &replicas
	_, err = client.AppsV1().Deployments(namespace).Update(deployment)
	if err != nil {
		return err
	}

	return nil
}
