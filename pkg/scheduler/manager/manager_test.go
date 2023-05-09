package manager

import (
	"testing"

	"github.com/superedge/orin-device-system/pkg/common"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPredicate(t *testing.T) {

	scache := NewScheduleCache()

	mng := NewManager(scache, nil)

	// 1111 means have 4 orin, which id is 1 2 3 4.
	q, _ := resource.ParseQuantity("1111")
	node1 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceName(common.ExtendResouceTypeBoardPrefix + "0"): q,
			},
		},
	}
	node2 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-2",
		},
		Status: v1.NodeStatus{},
	}

	mng.AddNode(node1)
	mng.AddNode(node2)

	reqQuan, _ := resource.ParseQuantity("1")
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "test",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceName(common.ExtendResouceTypeOrinPrefix + "1"): reqQuan,
						},
					},
				},
			},
		},
	}

	res, _, _ := mng.Predicate([]string{"node-1", "node-2"}, pod)
	if len(res) != 1 && res[0] != "node-1" {
		t.Fatalf("failed predicate, expect %s, actual %v", "node-1", res)
	}
}

func TestPriority(t *testing.T) {

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

	reqQuan, _ := resource.ParseQuantity("1")
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "test",
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceName(common.ExtendResouceTypeOrinPrefix + "1"): reqQuan,
						},
					},
				},
			},
		},
	}

	res := mng.Priority([]string{"node-1", "node-2"}, pod)
	expect := []int{9, 10}
	for i, s := range res {
		if s != expect[i] {
			t.Fatalf("failed priority, expect %d, actual %d", expect[i], s)
		}

	}
}
