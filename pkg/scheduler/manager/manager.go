package manager

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/avast/retry-go"
	"github.com/superedge/orin-device-system/pkg/common"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type Manager interface {
	// Predicate check node if has sufficent device resource to allocate
	Predicate(nodes []string, pod *v1.Pod) ([]string, map[string]string, error)
	// Priority score node for place current pod device resource request
	Priority(node []string, pod *v1.Pod) []int
	// Bind will update pod annotaion and update local cache
	Bind(node string, name, namespace string, podUID types.UID) error
	// GetPodFromApiserver use clientset to get newest pod info, instead localcache
	GetPodFromApiserver(podName, podNamespace string, podUID types.UID) (*v1.Pod, error)
	Cache
}

func NewManager(c Cache, clientSet kubernetes.Interface) *manager {
	return &manager{
		Cache:     c,
		ClientSet: clientSet,
	}
}

type manager struct {
	Cache
	ClientSet kubernetes.Interface
}

func (m *manager) GetPodFromApiserver(name, namespace string, podUID types.UID) (*v1.Pod, error) {
	pod, err := m.ClientSet.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if pod.UID != podUID {
		pod, err = m.ClientSet.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if pod.UID != podUID {
			return nil, fmt.Errorf("pod %s in ns %s's uid is %v, and it's not equal with expected %v",
				name,
				namespace,
				pod.UID,
				podUID)
		}
	}

	return pod, nil

}

func (m *manager) Predicate(nodes []string, pod *v1.Pod) ([]string, map[string]string, error) {
	klog.V(6).InfoS("before Predicate", "nodes", nodes, "podName", pod.Name)
	filterdNodes := make([]string, len(nodes))
	failNodes := make(map[string]string, len(nodes))
	var predicateResultLock sync.Mutex
	var filteredLen int32
	allocatorPolicy, ok := pod.Annotations[common.AnnotationPodBindOrinPolicy]
	if !ok {
		allocatorPolicy = AllocatorPolicyBinPack
	}
	allocator := AllocatorMap[allocatorPolicy]

	orinRequest := BuildRequestOrinSet(pod)
	checkNodes := func(i int) {
		nodeName := nodes[i]
		ni := m.Cache.GetNode(nodeName)
		if ni == nil {
			predicateResultLock.Lock()
			failNodes[nodeName] = nodeName + " not found in node cache"
			predicateResultLock.Unlock()
			return
		}

		res := allocator.Allocate(ni.Allocatable, orinRequest)
		klog.V(6).InfoS("allocator info",
			"policy", allocatorPolicy,
			"node", nodeName,
			"allocatable", ni.Allocatable,
			"request", orinRequest,
			"result", res,
		)

		if res.boardID != BoardIDNotFount {
			filterdNodes[atomic.AddInt32(&filteredLen, 1)-1] = nodes[i]
		} else {
			predicateResultLock.Lock()
			failNodes[nodeName] = nodeName + " has not enough resource"
			predicateResultLock.Unlock()
		}
	}
	Parallelize(16, len(nodes), checkNodes)
	klog.V(6).InfoS("after Predicate", "nodes", filterdNodes, "podName", pod.Name)
	return filterdNodes, failNodes, nil
}

func (m *manager) Priority(nodes []string, pod *v1.Pod) []int {
	allocatorPolicy, ok := pod.Annotations[common.AnnotationPodBindOrinPolicy]
	if !ok {
		allocatorPolicy = AllocatorPolicyBinPack
	}

	scores := make([]int, len(nodes))
	allocator := AllocatorMap[allocatorPolicy]
	orinRequest := BuildRequestOrinSet(pod)
	checkNodes := func(i int) {
		nodeName := nodes[i]
		ni := m.Cache.GetNode(nodeName)
		if ni == nil {
			scores[i] = 0
			return
		}
		res := allocator.Allocate(ni.Allocatable, orinRequest)
		if res.boardID != BoardIDNotFount {
			scores[i] = res.score
		} else {
			scores[i] = 0
		}
	}
	Parallelize(16, len(nodes), checkNodes)
	// Normalize score to 1-10
	Normalize(10, scores)
	klog.V(6).InfoS("after Priority", "scores", scores, "podName", pod.Name)
	return scores
}

func (m *manager) Bind(node string, podName, podNamespace string, podUID types.UID) error {

	ni := m.GetNode(node)
	if ni == nil {
		return fmt.Errorf("could not find bind node %s", node)
	}
	var newPod *v1.Pod
	bindPod := func() error {
		pod, err := m.GetPodFromApiserver(podName, podNamespace, podUID)
		if err != nil {
			return err
		}

		AllocatorPolicy, ok := pod.Annotations[common.AnnotationPodBindOrinPolicy]
		if !ok {
			AllocatorPolicy = AllocatorPolicyBinPack
		}
		allocator := AllocatorMap[AllocatorPolicy]
		orinRequest := BuildRequestOrinSet(pod)
		res := allocator.Allocate(ni.Allocatable, orinRequest)
		if res.boardID == BoardIDNotFount {
			return fmt.Errorf("could not find board %s", node)
		}
		newPod = AddPodBindAnnotation(pod, res.boardID)

		if err := m.AssumePod(newPod, node); err != nil {
			return err
		}

		if _, err := m.ClientSet.CoreV1().Pods(newPod.Namespace).Update(context.Background(), newPod, metav1.UpdateOptions{}); err != nil {
			if err := m.ForgetPod(pod, node); err != nil {
				klog.ErrorS(err, "forgot pod error", "pod name", pod.Name, "node name", node)
			}
			return err
		}
		return nil
	}
	if err := retry.Do(bindPod, retry.Attempts(3), retry.Delay(time.Second*1)); err != nil {
		return err
	}

	if err := m.ClientSet.CoreV1().Pods(podNamespace).Bind(context.Background(), &v1.Binding{
		ObjectMeta: metav1.ObjectMeta{Namespace: podNamespace, Name: podName, UID: podUID},
		Target: v1.ObjectReference{
			Kind: "Node",
			Name: node,
		},
	}, metav1.CreateOptions{}); err != nil {
		if err := m.ForgetPod(newPod, node); err != nil {
			klog.ErrorS(err, "forgot pod error", "pod name", newPod.Name, "node name", node)
		}
		recoverPod := RemovePodBindAnnotation(newPod)
		if _, err := m.ClientSet.CoreV1().Pods(newPod.Namespace).Update(context.Background(), recoverPod, metav1.UpdateOptions{}); err != nil {
			klog.ErrorS(err, "remove pod bin annotation error", "podname", recoverPod.Name)
			return err
		}
		return err
	}
	klog.V(6).Infof("update pod %s to pods cache", podName)

	return nil
}
