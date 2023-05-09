package manager

import (
	"testing"

	"github.com/superedge/orin-device-system/pkg/common"
	"github.com/superedge/orin-device-system/pkg/scheduler/manager/topo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddNode(t *testing.T) {

	ni1md := topo.NewBoardDetails()
	ni1md[0] = topo.NewOrinDetails().Add(0, 1).Add(0, 2).Add(0, 3).Add(0, 4)

	ni2md := topo.NewBoardDetails()
	ni2md[0] = topo.NewOrinDetails().Add(0, 1).Add(0, 4)

	scache := NewScheduleCache()

	mng := NewManager(scache, nil)

	// 1111 means have 4 orin, which id is 1 2 3 4.
	q1, _ := resource.ParseQuantity("1111")
	q2, _ := resource.ParseQuantity("1001")

	node1 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceName(common.ExtendResouceTypeBoardPrefix + "0"): q1,
			},
		},
	}
	node2 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-2",
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceName(common.ExtendResouceTypeBoardPrefix + "0"): q2,
			},
		},
	}

	mng.AddNode(node1)
	mng.AddNode(node2)

	ni1 := scache.nodeCache["node-1"]
	ni2 := scache.nodeCache["node-2"]

	if !ni1.Total.Equal(ni1md) {
		t.Fatalf("test add node error, expect %v, actual %v", ni1md, ni1.Total)
	}
	if !ni2.Total.Equal(ni2md) {
		t.Fatalf("test add node error, expect %v, actual %v", ni2md, ni2.Total)
	}
}

func TestAddPod(t *testing.T) {

	ni1md := topo.NewBoardDetails()
	ni1md[0] = topo.NewOrinDetails().Add(0, 4)

	ni2md := topo.NewBoardDetails()
	ni2md[0] = topo.NewOrinDetails().Add(0, 1).Add(0, 4)

	scache := NewScheduleCache()

	mng := NewManager(scache, nil)

	// 1111 means have 4 orin, which id is 1 2 3 4.
	q1, _ := resource.ParseQuantity("1111")

	node1 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceName(common.ExtendResouceTypeBoardPrefix + "0"): q1,
			},
		},
	}
	reqPod1, _ := resource.ParseQuantity("1")

	pod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pod-1",
			Namespace:   "default",
			Annotations: map[string]string{common.AnnotationPodBindToBoard: "0"},
		},
		Spec: v1.PodSpec{
			NodeName: "node-1",
			Containers: []v1.Container{
				{
					Name: "test",
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceName(common.ExtendResouceTypeOrinPrefix + "1"): reqPod1,
							v1.ResourceName(common.ExtendResouceTypeOrinPrefix + "2"): reqPod1,
							v1.ResourceName(common.ExtendResouceTypeOrinPrefix + "3"): reqPod1,
						},
					},
				},
			},
		},
	}

	mng.AddNode(node1)
	mng.AddPod(pod1)
	ni1 := scache.nodeCache["node-1"]

	if !ni1.Allocatable.Equal(ni1md) {
		t.Fatalf("test add node error, expect %v, actual %v", ni1md, ni1.Allocatable)
	}
}
