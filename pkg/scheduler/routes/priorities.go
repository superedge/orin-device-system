package routes

import (
	"github.com/superedge/orin-device-system/pkg/scheduler/manager"
	"k8s.io/klog/v2"
	schedulerapi "k8s.io/kube-scheduler/extender/v1"
)

type Prioritize struct {
	Name    string
	Manager manager.Manager
}

func (p Prioritize) Handler(args schedulerapi.ExtenderArgs) (*schedulerapi.HostPriorityList, error) {
	priorityList := make(schedulerapi.HostPriorityList, len(*args.NodeNames))
	scores := p.Manager.Priority(*args.NodeNames, args.Pod)
	for i, score := range scores {
		priorityList[i] = schedulerapi.HostPriority{
			Host:  (*args.NodeNames)[i],
			Score: int64(score),
		}
	}
	klog.V(5).InfoS("Prioritize scores", "score", priorityList)
	return &priorityList, nil
}

func NewPrioritize(name string, m manager.Manager) Prioritize {
	return Prioritize{Name: name, Manager: m}
}
