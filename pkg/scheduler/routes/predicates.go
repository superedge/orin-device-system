package routes

import (
	"github.com/superedge/orin-device-system/pkg/scheduler/manager"
	"k8s.io/klog/v2"
	schedulerapi "k8s.io/kube-scheduler/extender/v1"
)

type Predicate struct {
	Name    string
	Manager manager.Manager
}

func (p Predicate) Handler(args schedulerapi.ExtenderArgs) *schedulerapi.ExtenderFilterResult {
	klog.V(5).InfoS("predicates args", "args", args)
	filterdNodes, faildNodes, err := p.Manager.Predicate(*args.NodeNames, args.Pod)
	if err != nil {
		return &schedulerapi.ExtenderFilterResult{
			Error: err.Error(),
		}
	}

	result := schedulerapi.ExtenderFilterResult{
		NodeNames:   &filterdNodes,
		FailedNodes: faildNodes,
		Error:       "",
	}

	return &result
}

func NewPredicate(name string, m manager.Manager) Predicate {
	return Predicate{Name: name, Manager: m}
}
