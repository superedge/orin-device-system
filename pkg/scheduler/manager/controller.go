package manager

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/superedge/orin-device-system/pkg/common"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	clientgocache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

var (
	KeyFunc      = clientgocache.DeletionHandlingMetaNamespaceKeyFunc
	resyncPeriod = 30 * time.Second
)

type Controller struct {
	manager   Manager
	clientset kubernetes.Interface

	// podLister can list/get pods from the shared informer's store.
	podLister corelisters.PodLister

	// nodeLister can list/get nodes from the shared informer's store.
	nodeLister corelisters.NodeLister

	// podQueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	podQueue workqueue.RateLimitingInterface

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// podInformerSynced returns true if the pod store has been synced at least once.
	podInformerSynced clientgocache.InformerSynced

	// nodeInformerSynced returns true if the service store has been synced at least once.
	nodeInformerSynced clientgocache.InformerSynced
}

func NewController(clientset kubernetes.Interface, manager Manager, stopCh <-chan struct{}) (c *Controller, err error) {
	informerFactory := informers.NewSharedInformerFactory(clientset, resyncPeriod)

	klog.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "orin-device-system"})

	c = &Controller{
		clientset: clientset,
		podQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "podQueue"),
		recorder:  recorder,
	}
	// Create pod informer.
	podInformer := informerFactory.Core().V1().Pods()
	podInformer.Informer().AddEventHandler(clientgocache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *v1.Pod:
				return IsOrinPod(t)
			case clientgocache.DeletedFinalStateUnknown:
				if pod, ok := t.Obj.(*v1.Pod); ok {
					klog.Infof("delete pod %s/%s", pod.Namespace, pod.Name)
					return IsOrinPod(pod)
				}
				runtime.HandleError(fmt.Errorf("unable to convert object %T to *v1.Pod in %T", obj, c))
				return false
			default:
				runtime.HandleError(fmt.Errorf("unable to handle object in %T: %T", c, obj))
				return false
			}
		},
		Handler: clientgocache.ResourceEventHandlerFuncs{
			AddFunc:    c.addPodToCache,
			UpdateFunc: c.updatePodInCache,
			DeleteFunc: c.deletePodFromCache,
		},
	})

	c.podLister = podInformer.Lister()
	c.podInformerSynced = podInformer.Informer().HasSynced

	// Create node informer
	nodeInformer := informerFactory.Core().V1().Nodes()
	nodeInformer.Informer().AddEventHandler(clientgocache.ResourceEventHandlerFuncs{
		AddFunc:    c.addNodeToCache,
		UpdateFunc: c.updateNodeInCache,
		DeleteFunc: c.deleteNodeFromCache,
	})
	c.nodeLister = nodeInformer.Lister()
	c.nodeInformerSynced = nodeInformer.Informer().HasSynced

	c.manager = manager
	// Start informer goroutines.
	go informerFactory.Start(stopCh)

	klog.Info("begin to wait for cache")

	if ok := clientgocache.WaitForCacheSync(stopCh, c.nodeInformerSynced); !ok {
		return nil, fmt.Errorf("failed to wait for node caches to sync")
	} else {
		klog.Info("init the node cache successfully")
	}

	if ok := clientgocache.WaitForCacheSync(stopCh, c.podInformerSynced); !ok {
		return nil, fmt.Errorf("failed to wait for pod caches to sync")
	} else {
		klog.Info("init the pod cache successfully")
	}

	klog.Info("end to wait for cache")

	return c, nil
}

// Run will set up the event handlers
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.podQueue.ShutDown()

	klog.Info("Starting orin device controller.")
	klog.Info("Waiting for informer caches to sync")

	klog.Infof("Starting %v workers.", threadiness)
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// syncPod will sync the pod with the given key if it has had its expectations fulfilled,
// meaning it did not expect to see any more of its pods created or deleted. This function is not meant to be
// invoked concurrently with the same key.
func (c *Controller) syncPod(key string) (forget bool, err error) {
	klog.V(2).Infof("begin to sync controller for pod %s", key)
	ns, name, err := clientgocache.SplitMetaNamespaceKey(key)
	if err != nil {
		return false, err
	}

	pod, err := c.podLister.Pods(ns).Get(name)
	switch {
	case errors.IsNotFound(err):
		klog.V(2).Infof("pod %s/%s has been deleted.", ns, name)
	case err != nil:
		klog.Warningf("unable to retrieve pod %v from the store: %v", key, err)
	default:
		if IsCompletedPod(pod) {
			klog.V(2).Infof("pod %s/%s has completed.", ns, name)
			if err := c.releasePod(pod); err != nil {
				klog.Errorf("release pod %s/%s failed: %s", pod.Namespace, pod.Name, err.Error())
			}
		} else {
			if pod.Spec.NodeName == "" {
				return true, nil
			}
			err := c.assignPod(pod)
			if err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

// processNextWorkItem will read a single work item off the podQueue and
// attempt to process it.
func (c *Controller) processNextWorkItem() bool {
	klog.V(4).Info("begin processNextWorkItem()")
	key, quit := c.podQueue.Get()
	if quit {
		return false
	}
	defer c.podQueue.Done(key)
	defer klog.V(4).Info("end processNextWorkItem()")
	forget, err := c.syncPod(key.(string))
	if err == nil {
		if forget {
			c.podQueue.Forget(key)
		}
		return false
	}

	klog.Infof("Error syncing pods: %v", err)
	runtime.HandleError(fmt.Errorf("error syncing pod: %v", err))
	c.podQueue.AddRateLimited(key)

	return true
}

func (c *Controller) addPodToCache(obj interface{}) {
	startTime := time.Now()
	klog.V(6).InfoS("start addPodToCache")
	defer func() {
		klog.V(6).InfoS("finish addPodToCache", "duration", time.Since(startTime))
	}()
	pod, ok := obj.(*v1.Pod)
	if !ok {
		klog.Warningf("cannot convert to *v1.Pod: %v", obj)
		return
	}

	podKey, err := KeyFunc(pod)
	if err != nil {
		klog.Warningf("Failed to get the jobkey: %v", err)
		return
	}

	c.podQueue.Add(podKey)
}

func (c *Controller) updatePodInCache(oldObj, newObj interface{}) {
	startTime := time.Now()
	klog.V(6).InfoS("start updatePodInCache")
	defer func() {
		klog.V(6).InfoS("finish updatePodInCache", "duration", time.Since(startTime))
	}()

	oldPod, ok := oldObj.(*v1.Pod)
	if !ok {
		klog.Warningf("cannot convert oldObj to *v1.Pod: %v", oldObj)
		return
	}
	newPod, ok := newObj.(*v1.Pod)
	if !ok {
		klog.Warningf("cannot convert newObj to *v1.Pod: %v", newObj)
		return
	}
	if oldPod.ResourceVersion == newPod.ResourceVersion {
		return
	}

	// find completed pod which binding board
	if c.manager.KnownPod(oldPod) && IsCompletedPod(newPod) && IsBindingBoard(newPod) {
		podKey, err := KeyFunc(newPod)
		if err != nil {
			klog.Warningf("Failed to get the job key: %v", err)
			return
		}
		klog.V(2).Infof("Need to update pod name %s/%s and old status is %v, new status is %v; its old annotation %v and new annotation %v",
			newPod.Namespace,
			newPod.Name,
			oldPod.Status.Phase,
			newPod.Status.Phase,
			oldPod.Annotations,
			newPod.Annotations)
		c.podQueue.Add(podKey)

	} else {
		klog.V(4).Infof("No need to update pod name %s/%s and old status is %v, new status is %v; its old annotation %v and new annotation %v",
			newPod.Namespace,
			newPod.Name,
			oldPod.Status.Phase,
			newPod.Status.Phase,
			oldPod.Annotations,
			newPod.Annotations)
	}

}

func (c *Controller) deletePodFromCache(obj interface{}) {
	startTime := time.Now()
	klog.V(6).InfoS("start deletePodFromCache")
	defer func() {
		klog.V(6).InfoS("finish deletePodFromCache", "duration", time.Since(startTime))
	}()

	var pod *v1.Pod
	switch t := obj.(type) {
	case *v1.Pod:
		pod = t
	case clientgocache.DeletedFinalStateUnknown:
		var ok bool
		pod, ok = t.Obj.(*v1.Pod)
		if !ok {
			klog.Warningf("cannot convert to *v1.Pod: %v", t.Obj)
			return
		}
	default:
		klog.Warningf("cannot convert to *v1.Pod: %v", t)
		return
	}

	klog.Infof("delete pod %s/%s", pod.Namespace, pod.Name)

	c.releasePod(pod)
}

func (c *Controller) addNodeToCache(obj interface{}) {
	startTime := time.Now()
	klog.V(6).InfoS("start addNodeToCache")
	defer func() {
		klog.V(6).InfoS("finish addNodeToCache", "duration", time.Since(startTime))
	}()

	node := obj.(*v1.Node)
	if node.DeletionTimestamp != nil {
		// On a restart of the controller manager, it's possible for an object to
		// show up in a state that is already pending deletion.
		if err := c.manager.DeleteNode(node); err != nil {
			klog.ErrorS(err, "manager.DeleteNode error", "nodename", node.Name)
		}
		return
	}
	pods, err := c.getActivePodInNode(node.Name)
	if err != nil {
		klog.ErrorS(err, "get active pod error", "nodename", node.Name)
	}

	if err := c.manager.AddNode(node, pods...); err != nil {
		klog.ErrorS(err, "manager.AddNode error", "nodename", node.Name)
	}
}
func (c *Controller) updateNodeInCache(obj interface{}, newobj interface{}) {
	startTime := time.Now()
	klog.V(6).InfoS("start updateNodeInCache")
	defer func() {
		klog.V(6).InfoS("finish updateNodeInCache", "duration", time.Since(startTime))
	}()

	oldNode := obj.(*v1.Node)
	newNode := newobj.(*v1.Node)
	if newNode.DeletionTimestamp != nil {
		// On a restart of the controller manager, it's possible for an object to
		// show up in a state that is already pending deletion.
		if err := c.manager.DeleteNode(newNode); err != nil {
			klog.ErrorS(err, "manager.DeleteNode error", "nodename", newNode.Name)
		}
		return
	}
	pods, err := c.getActivePodInNode(newNode.Name)
	if err != nil {
		klog.ErrorS(err, "get active pod error", "nodename", newNode.Name)
	}

	if err := c.manager.UpdateNode(oldNode, newNode, pods...); err != nil {
		klog.ErrorS(err, "manager.UpdateNode error", "node name", newNode.Name)
	}
}

func (c *Controller) deleteNodeFromCache(obj interface{}) {
	startTime := time.Now()
	klog.V(6).InfoS("start deleteNodeFromCache")
	defer func() {
		klog.V(6).InfoS("finish deleteNodeFromCache", "duration", time.Since(startTime))
	}()

	node, ok := obj.(*v1.Node)
	if !ok {
		tombstone, ok := obj.(clientgocache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		node, ok = tombstone.Obj.(*v1.Node)
		if !ok {
			runtime.HandleError(fmt.Errorf("tombstone contained object is not a node %#v", obj))
			return
		}
	}

	if err := c.manager.DeleteNode(node); err != nil {
		klog.ErrorS(err, "manager.AddNode error", "nodename", node.Name)
	}
}

func (c *Controller) releasePod(pod *v1.Pod) error {
	return c.manager.DeletePod(pod)
}

// func (c *Controller) releasedPod(pod *v1.Pod) bool {
// 	d, err := scheduler.GetResourceScheduler(pod, c.RegisteredSchedulers)
// 	if err != nil {
// 		return false
// 	}
// 	return d.ReleasedPod(pod)
// }

// func (c *Controller) knownPod(pod *v1.Pod) bool {
// 	d, err := scheduler.GetResourceScheduler(pod, c.RegisteredSchedulers)
// 	if err != nil {
// 		return false
// 	}
// 	return d.KnownPod(pod)
// }

func (c *Controller) assignPod(pod *v1.Pod) error {
	_, ok := pod.Annotations[common.AnnotationPodBindToBoard]
	if !ok {
		return nil
	}

	// first check if pod node exist in cache, this may happened in orin scheduler restart
	// if node is not in cache, we should add a nodeinfo first
	ni := c.manager.GetNode(pod.Spec.NodeName)
	if ni == nil {
		// node info not in cache, create nodeinfo first
		node, err := c.clientset.CoreV1().Nodes().Get(context.Background(), pod.Spec.NodeName, metav1.GetOptions{})
		if err != nil {
			klog.ErrorS(err, "get node from apiserver error", "nodename", node.Name)
		} else {
			pods, err := c.getActivePodInNode(node.Name)
			if err != nil {
				klog.ErrorS(err, "get active pod error", "nodename", node.Name)

			}
			if err := c.manager.AddNode(node, pods...); err != nil {
				klog.ErrorS(err, "cache add node error", "nodename", node.Name)
			}
		}

	}

	return c.manager.AddPod(pod)
}

func (c *Controller) getActivePodInNode(nodeName string) ([]*v1.Pod, error) {
	res := make([]*v1.Pod, 0, 20)
	podList, err := c.podLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, p := range podList {
		if p.Spec.NodeName == nodeName && !IsCompletedPod(p) && IsBindingBoard(p) {
			res = append(res, p)
		}
	}

	return res, nil
}
func IsOrinPod(pod *v1.Pod) bool {
	if IsResourceExists(pod, common.ExtendResouceTypeBoard) && IsResourceExists(pod, common.ExtendResouceTypeOrinPrefix) {
		return true
	}
	return false
}

func IsResourceExists(pod *v1.Pod, resourceName string) bool {
	for _, c := range pod.Spec.Containers {
		for rkey := range c.Resources.Limits {
			if strings.HasPrefix(string(rkey), resourceName) {
				return true
			}
		}
	}
	return false
}

func IsCompletedPod(pod *v1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return true
	}

	if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
		return true
	}
	return false
}

func IsBindingBoard(pod *v1.Pod) bool {
	_, ok := pod.Annotations[common.AnnotationPodBindToBoard]
	return ok
}
