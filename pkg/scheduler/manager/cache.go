package manager

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/superedge/orin-device-system/pkg/common"
	"github.com/superedge/orin-device-system/pkg/scheduler/manager/topo"
	v1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

type NodeInfo struct {
	// Overall node information.
	Node *v1.Node

	// Pods running on the node.
	Pods map[types.UID]*v1.Pod

	// Requested is orin device num
	Allocatable topo.BoardDetails
	Requested   topo.BoardDetails
	Total       topo.BoardDetails
}

// addPod only focus pod which has bind to node and board
func (ni *NodeInfo) addPod(pod *v1.Pod) error {
	bindBoardIDStr, ok := pod.Annotations[common.AnnotationPodBindToBoard]
	if !ok {
		return nil
	}
	bindBoardID, err := strconv.ParseInt(bindBoardIDStr, 10, 0)
	if err != nil {
		klog.ErrorS(err, "find a invalid pod bind boardID", "annotation value", bindBoardIDStr)
		return nil
	}
	// pod has exist in nodeinfo cache, update it
	if _, ok := ni.Pods[pod.UID]; ok {
		if err := ni.deletePod(pod); err != nil {
			return err
		}
	}
	// 1. check pod has orin request
	orinSet := BuildRequestOrinSet(pod)
	var od topo.OrinDetails
	// 2. caculate request
	if existOd, ok := ni.Requested[int(bindBoardID)]; ok {
		od = existOd
	} else {
		od = topo.NewOrinDetails()
	}
	for _, o := range orinSet.UnsortedList() {
		od.Add(int(bindBoardID), o)
	}
	ni.Requested[int(bindBoardID)] = od
	// 3. caculate allocatable
	newAllocatable, err := ni.Total.DifferenceFromSuperset(ni.Requested)
	if err != nil {
		klog.ErrorS(err, "Caculate node allocatable error", "nodename", ni.Node.Name, "total", ni.Total, "request", ni.Requested)
		return err
	}
	ni.Allocatable = newAllocatable
	// 4. add pod to cache
	ni.Pods[pod.UID] = pod
	klog.V(4).InfoS("after node info add pod", "node name", ni.Node.Name, "pod name", pod.Name, "total details", ni.Total, "request", ni.Requested, "allocatable", ni.Allocatable)

	return nil
}
func (ni *NodeInfo) deletePod(pod *v1.Pod) error {

	bindBoardIDStr, ok := pod.Annotations[common.AnnotationPodBindToBoard]
	if !ok {
		return nil
	}

	if _, ok := ni.Pods[pod.UID]; !ok {
		return fmt.Errorf("node %s has not contain pod %s,uid %v", ni.Node.Name, pod.Name, pod.UID)
	}

	bindBoardID, err := strconv.ParseInt(bindBoardIDStr, 10, 0)
	if err != nil {
		klog.ErrorS(err, "find a invalid pod bind boardID", "annotation value", bindBoardIDStr)
		return nil
	}
	// 1. sum pod orin request
	needReleaseOrinSet := BuildRequestOrinSet(pod)
	needReleaseOrinDetails := topo.NewOrinDetails()

	for _, o := range needReleaseOrinSet.UnsortedList() {
		needReleaseOrinDetails.Add(int(bindBoardID), o)
	}

	needReleaseBoardDetails := topo.NewBoardDetails().Add(int(bindBoardID), needReleaseOrinDetails)
	// 2. caculate request
	newRequest, err := ni.Requested.DifferenceFromSuperset(needReleaseBoardDetails)
	if err != nil {
		klog.V(6).ErrorS(err, "Caculate node request error", "nodename", ni.Node.Name, "request", ni.Requested, "needRelease", needReleaseBoardDetails)
		return err
	}
	ni.Requested = newRequest
	// 3. caculate allocatable
	ni.Allocatable.Add(int(bindBoardID), needReleaseOrinDetails)

	// TODO need check request plus allocatable is equal to total
	// 4. delete pod in cache
	delete(ni.Pods, pod.UID)

	klog.V(4).InfoS("after node info delete pod", "node name", ni.Node.Name, "pod name", pod.Name, "total details", ni.Total, "request", ni.Requested, "allocatable", ni.Allocatable)
	return nil

}

// func (ni *NodeInfo) updatePod(pod *v1.Pod) error {
// 	_, ok := pod.Annotations[common.AnnotationPodBindToBoard]
// 	if !ok {
// 		return nil
// 	}
// 	// current we do not support pod update request and pod and pod change board bind directly by manual
// 	return nil

// }

type Cache interface {
	// pod cache will be updated by two way, one is Bind() method, the other is
	// pod informer call back handler
	// AddPod add a pod to cache which we focus on
	AddPod(pod *v1.Pod) error
	// DeletePod add a pod to cache which we focus on
	DeletePod(pod *v1.Pod) error

	KnownPod(pod *v1.Pod) bool
	// node cache will be updated by node informer call back handler
	// AddNode add a node to cache
	AddNode(node *v1.Node, pods ...*v1.Pod) error
	// UpdateNode update a node in cache, when device-plugin check orin device offline
	// it will be update board and orin extend resources
	GetNode(name string) *NodeInfo
	UpdateNode(oldNode *v1.Node, newNode *v1.Node, pods ...*v1.Pod) error
	// DeleteNode delete a node to cache
	DeleteNode(node *v1.Node) error
	// AssumePod will add pod to tmp cache
	AssumePod(pod *v1.Pod, node string) error
	// ForgetPod will clear assume cache, like bind error
	ForgetPod(pod *v1.Pod, node string) error
}

func NewScheduleCache() *scheduleCache {
	return &scheduleCache{
		nodeCache:   make(map[string]*NodeInfo),
		assumePods:  sets.NewString(),
		mu:          new(sync.RWMutex),
		podMaps:     make(map[types.UID]*v1.Pod),
		releasedPod: make(map[types.UID]struct{}),
	}
}

type scheduleCache struct {
	nodeCache   map[string]*NodeInfo
	assumePods  sets.String
	podMaps     map[types.UID]*v1.Pod
	releasedPod map[types.UID]struct{}

	mu *sync.RWMutex
}

func (c *scheduleCache) AddPod(pod *v1.Pod) error {
	// find pod that use orin and has been scheduled
	if pod.Spec.NodeName == "" {
		return nil
	}
	_, ok := pod.Annotations[common.AnnotationPodBindToBoard]
	if !ok {
		return nil
	}
	c.mu.Lock()

	defer c.mu.Unlock()
	if _, ok := c.podMaps[pod.UID]; ok {
		return nil
	}

	// if node is not in cache ingore
	// TODO deal like kube-scheduler?
	ni, ok := c.nodeCache[pod.Spec.NodeName]
	if !ok {
		return nil
	}
	// update node info cache
	if err := ni.addPod(pod); err != nil {
		return err
	}
	c.podMaps[pod.UID] = pod

	return nil
}

func (c *scheduleCache) DeletePod(pod *v1.Pod) error {
	// find pod that use orin and has been scheduled
	if pod.Spec.NodeName == "" {
		return nil
	}
	_, ok := pod.Annotations[common.AnnotationPodBindToBoard]
	if !ok {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// if node is not in cache ingore
	// TODO deal like kube-scheduler?
	ni, ok := c.nodeCache[pod.Spec.NodeName]
	if !ok {
		return nil
	}
	err := ni.deletePod(pod)
	if err != nil {
		return err
	}
	if _, ok := c.podMaps[pod.UID]; ok {
		delete(c.podMaps, pod.UID)
		c.releasedPod[pod.UID] = struct{}{}
	}

	return err
}
func (c *scheduleCache) AddNode(node *v1.Node, pods ...*v1.Pod) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ni := NewNodeInfo(node, pods...)
	if ni != nil {
		c.nodeCache[node.Name] = ni
	}

	return nil
}
func (c *scheduleCache) GetNode(name string) *NodeInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.nodeCache[name]
}

func (c *scheduleCache) UpdateNode(oldNode *v1.Node, newNode *v1.Node, pods ...*v1.Pod) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// check if need recreate node info

	oldNi := NewNodeInfo(oldNode, pods...)
	newNi := NewNodeInfo(newNode, pods...)

	if !oldNi.Total.Equal(newNi.Total) {
		klog.V(2).InfoS("find board device update, need update cache", "nodename", newNode.Name)
		// copy old node info's resource
		newNi.Requested = oldNi.Requested
		// new node resource can not cover old pod request, so set node allocatable field zero
		if newAllocate, err := newNi.Total.DifferenceFromSuperset(newNi.Requested); err != nil {
			newNi.Allocatable = topo.NewBoardDetails()
		} else {
			newNi.Allocatable = newAllocate
		}
		c.nodeCache[newNode.Name] = newNi
	}
	return nil
}
func (c *scheduleCache) DeleteNode(node *v1.Node) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.nodeCache[node.Name]
	if !ok {
		return fmt.Errorf("node %v is not found", node.Name)
	}

	delete(c.nodeCache, node.Name)

	return nil
}

func (c *scheduleCache) AssumePod(pod *v1.Pod, node string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	ni, ok := c.nodeCache[node]
	if !ok {
		return fmt.Errorf("node %v is not found", node)
	}
	if _, ok := c.podMaps[pod.UID]; ok {
		return fmt.Errorf("pod %v(%v) is in the cache, so can't be assumed", pod.Name, klog.KObj(pod))
	}
	if err := ni.addPod(pod); err != nil {
		return err
	}
	c.assumePods.Insert(string(pod.UID))
	c.podMaps[pod.UID] = pod
	return nil
}

func (c *scheduleCache) ForgetPod(pod *v1.Pod, node string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	ni, ok := c.nodeCache[node]
	if !ok {
		return fmt.Errorf("node %v is not found", node)
	}
	if _, ok := c.podMaps[pod.UID]; !ok {
		return fmt.Errorf("pod %v(%v) is not in the cache, so can't be forgot", pod.Name, klog.KObj(pod))
	}

	if err := ni.deletePod(pod); err != nil {
		return err
	}

	return nil
}
func (c *scheduleCache) KnownPod(pod *v1.Pod) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.podMaps[pod.UID]
	return ok
}

func NewNodeInfo(node *v1.Node, pods ...*v1.Pod) *NodeInfo {
	klog.V(8).InfoS("new node info", "node", node.String())

	ni := &NodeInfo{}
	ni.Node = node
	ni.Pods = make(map[types.UID]*v1.Pod)
	totalBoardDetails := topo.NewBoardDetails()
	// caculate total
	for rkey, rval := range node.Status.Capacity {
		if strings.HasPrefix(string(rkey), common.ExtendResouceTypeBoardPrefix) {
			klog.V(6).InfoS("node board capacity", "key", rkey, "value", rval.Value())

			if boardID, err := BoardIDFromResourceName(string(rkey)); err != nil {
				klog.ErrorS(err, "find a invalid request", "request", rkey)
				continue
			} else {
				orinSet, err := NewDecimalMap(rval.Value(), DefaultOrinStartBit).Parse()
				if err != nil {
					klog.ErrorS(err, "find a invalid board resource value", "resource value", rval.Value())
					continue
				}
				klog.V(6).InfoS("after decimal map parse", "boardID", boardID, "orins", orinSet)

				od := topo.NewOrinDetails()
				for _, o := range orinSet.UnsortedList() {
					od.Add(int(boardID), o)
				}
				totalBoardDetails.Add(int(boardID), od)
			}
		}
	}
	ni.Total = totalBoardDetails
	ni.Requested = topo.NewBoardDetails()
	ni.Allocatable = ni.Total

	for _, p := range pods {
		if err := ni.addPod(p); err != nil {
			klog.ErrorS(err, "new NodeInfo find a invalid pod", "podname", p.Name)
		}
	}
	klog.V(6).InfoS("new node info", "node name", node.Name, "total details", ni.Total, "request", ni.Requested, "allocatable", ni.Allocatable)

	return ni

}

func BuildRequestOrinSet(pod *v1.Pod) sets.Int {
	orinSet := sets.NewInt()
	for _, c := range pod.Spec.Containers {
		for rkey := range c.Resources.Limits {
			if strings.HasPrefix(string(rkey), common.ExtendResouceTypeOrinPrefix) {
				if orinID, err := OrinIDFromResourceName(string(rkey)); err != nil {
					klog.ErrorS(err, "find a invalid request", "request", rkey)
					continue
				} else {
					orinSet.Insert(int(orinID))
				}
			}
		}
	}
	return orinSet
}
