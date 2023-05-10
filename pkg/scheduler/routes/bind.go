package routes

import (
	"github.com/superedge/orin-device-system/pkg/scheduler/manager"
	schedulerapi "k8s.io/kube-scheduler/extender/v1"
)

type Bind struct {
	Name    string
	Manager manager.Manager
}

func (b Bind) Handler(args schedulerapi.ExtenderBindingArgs) *schedulerapi.ExtenderBindingResult {
	errMsg := ""
	err := b.Manager.Bind(args.Node, args.PodName, args.PodNamespace, args.PodUID)
	if err != nil {
		errMsg = err.Error()
	}
	return &schedulerapi.ExtenderBindingResult{
		Error: errMsg,
	}
}
func NewBind(name string, m manager.Manager) Bind {
	return Bind{Name: name, Manager: m}
}
