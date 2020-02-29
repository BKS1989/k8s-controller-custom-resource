package main

import (
	"fmt"
	samplecrdv1 "github.com/bks1989/k8s-controller-custom-resource/pkg/apis/samplecrd/v1"
	clientset "github.com/bks1989/k8s-controller-custom-resource/pkg/client/clientset/versioned"
	networkscheme "github.com/bks1989/k8s-controller-custom-resource/pkg/client/clientset/versioned/scheme"
	informers "github.com/bks1989/k8s-controller-custom-resource/pkg/client/informers/externalversions/samplecrd/v1"
	listers "github.com/bks1989/k8s-controller-custom-resource/pkg/client/listers/samplecrd/v1"
	glog "github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	runtime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"time"
)

const (
	controllerAgentName = "network-controller"
)

const (
	SuccessSynced         = "Synced"
	MessageResourceSynced = "Network sync successfully"
)

type Controller struct {
	// kubernetesClientSet is standard kubernetes client set
	kubernetesClientSet kubernetes.Interface
	// networksClientSet is a clients for owen API
	networksClientSet clientset.Interface
	networksLister    listers.NetworkLister
	networksSynced    cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	// recorder is a event recorder for recording event resources to
	// kubernetes API
	recorder record.EventRecorder
}

//NewController return a new Network Controller
func NewController(
	kubernetesClientSet kubernetes.Interface,
	networkClientSet clientset.Interface,
	networkInformer informers.NetworkInformer) *Controller {

	// Create event broadcaster
	// Add network-controller types to default kubernetes scheme so
	// Event can be logged if network-controller types
	runtime.Must(networkscheme.AddToScheme(scheme.Scheme))
	glog.V(4).Info("Create event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubernetesClientSet.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	controller := &Controller{
		kubernetesClientSet: kubernetesClientSet,
		networksClientSet:   networkClientSet,
		networksLister:      networkInformer.Lister(),
		networksSynced:      networkInformer.Informer().HasSynced,
		workqueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Networks"),
		recorder:            recorder,
	}

	glog.Info("Setting event handlers")

	// set up an event handler when network resource change

	networkInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueceNetwork,
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldNetwork := oldObj.(*samplecrdv1.Network)
			newNetwork := newObj.(*samplecrdv1.Network)

			if oldNetwork.ResourceVersion == newNetwork.ResourceVersion {
				// Periodic resync will send update events for all known Networks.
				// Two different versions of the same Network will always have different RVs.
				return
			}
			controller.enqueceNetwork(newObj)
		},
		DeleteFunc: controller.enqueceNetworkForDelete,
	})

	return controller
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// start the informers factories to begin populate the informer cache
	glog.Info("starting Network Control Loop")

	// wait for the caches to be synced before starting worker
	glog.Info("Waiting for informer cache to be sync")

	if ok := cache.WaitForCacheSync(stopCh, c.networksSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")

	// Launch two workers to process Network resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Starting workers")

	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

func (c Controller) runWorker() {
	for c.processNextWorkItems() {

	}
}

// processNextWorkItems will read a single work items off the workquence
// attempts to process it , by calling the syncHandler
func (c Controller) processNextWorkItems() bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}
	// We wrap this block in a func so we can defer c.workquece.Done
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workquence we go %v", obj))
			return nil
		}

		// Run syncHandler, passing it the namespace/name string of network resource to be sync
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		return nil
	}(obj)
	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)

	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key %s", key))
	}

	// get network resource with namesapce/name
	network, err := c.networksLister.Networks(namespace).Get(name)

	if err != nil {
		if errors.IsNotFound(err) {
			glog.Warning("Network %s/%s does not exist in local cache, will delete it from Network", namespace, name)
			glog.Infof("[Neutron] Deleting network: %s/%s ...", namespace, name)
			return nil
		}
		runtime.HandleError(fmt.Errorf("failed to list network by %s:%s", namespace, name))
		return err
	}

	glog.Infof("Try to process network: %v ", network)
	c.recorder.Event(network, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c Controller) enqueceNetwork(obj interface{}) {
	var key string
	var err error

	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

func (c Controller) enqueceNetworkForDelete(obj interface{}) {
	var key string
	var err error
	key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}
