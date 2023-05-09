package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/superedge/orin-device-system/pkg/common"
	"github.com/superedge/orin-device-system/pkg/device/kubeapis"
	"github.com/superedge/orin-device-system/pkg/device/provider"
	"github.com/superedge/orin-device-system/pkg/device/types"
	"github.com/superedge/orin-device-system/pkg/scheduler/manager"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/core/v1"
)

const (
	HostVitualPath = "/data/edge/device"
)

type OrinDeviceConfig struct {
	DeviceLocator  map[v1.ResourceName]kubeapis.DeviceLocator
	Sitter         kubeapis.Sitter
	DeviceProvider provider.DeviceProvider
	NodeName       string
	ClientSet      *kubernetes.Clientset
}

type OrinDevicePlugin map[string]*DevicePluginServer

func NewOrinDevicePlugin(c *OrinDeviceConfig) (OrinDevicePlugin, error) {
	classes := c.DeviceProvider.GetOrinClasses()

	klog.V(5).InfoS("get devices from provider", "device ids", classes)
	odp := make(map[string]*DevicePluginServer, len(classes))
	// provider get orin soc ids

	for orinID, boardIDSets := range classes {
		resourceName := fmt.Sprintf("%s%d", common.ExtendResouceTypeOrinPrefix, orinID)
		c.DeviceLocator[v1.ResourceName(resourceName)] = kubeapis.NewKubeletDeviceLocator(resourceName)
		gsrv, err := NewOrinDeviceGrpcServer(c, resourceName, orinID, boardIDSets)
		if err != nil {
			return nil, err
		}
		odp[resourceName] = &DevicePluginServer{
			Endpoint:           fmt.Sprintf("%s.sock", strings.ReplaceAll(resourceName, "/", "-")),
			ResourceName:       resourceName,
			DevicePluginServer: gsrv,
		}
	}
	klog.V(5).InfoS("create orin device plugin", "odp", odp)
	// patch node extra resource
	if err := patchNodeExtraResource(c.ClientSet, c.DeviceProvider, c.NodeName); err != nil {
		return nil, err
	}
	return odp, nil
}

func (odp OrinDevicePlugin) Run(stop <-chan struct{}) {
	for _, p := range odp {
		klog.InfoS("start plugin", "name", p.ResourceName)
		go p.Run(stop)
	}
}

type OrinDeviceGrpcServer struct {
	OrinID       int
	ResourceName v1.ResourceName
	devices      []*v1beta1.Device
	*OrinDeviceConfig
}

func NewOrinDeviceGrpcServer(c *OrinDeviceConfig, resourceName string, orinID int, boardIDSets sets.Int) (*OrinDeviceGrpcServer, error) {

	devices := make([]*v1beta1.Device, boardIDSets.Len())

	for i, bid := range boardIDSets.UnsortedList() {
		devices[i] = &v1beta1.Device{
			ID:     fmt.Sprintf("%d-%d", bid, orinID),
			Health: v1beta1.Healthy,
		}
	}

	return &OrinDeviceGrpcServer{
		OrinID:           orinID,
		ResourceName:     v1.ResourceName(resourceName),
		devices:          devices,
		OrinDeviceConfig: c,
	}, nil

}

func (s *OrinDeviceGrpcServer) GetDevicePluginOptions(ctx context.Context, empty *v1beta1.Empty) (*v1beta1.DevicePluginOptions, error) {
	return &v1beta1.DevicePluginOptions{
		PreStartRequired: true,
	}, nil
}

func (s *OrinDeviceGrpcServer) ListAndWatch(empty *v1beta1.Empty, server v1beta1.DevicePlugin_ListAndWatchServer) error {
	if err := server.Send(&v1beta1.ListAndWatchResponse{Devices: s.devices}); err != nil {
		return err
	}
	<-server.Context().Done()
	return nil
}

func (s *OrinDeviceGrpcServer) PreStartContainer(ctx context.Context, request *v1beta1.PreStartContainerRequest) (*v1beta1.PreStartContainerResponse, error) {

	devicesIDs := request.DevicesIDs
	if len(devicesIDs) == 0 {
		return &v1beta1.PreStartContainerResponse{}, fmt.Errorf("devices is empty")
	}
	// get pod by device ids
	orindevice := types.NewDevice(devicesIDs, s.ResourceName)
	curr, err := s.DeviceLocator[s.ResourceName].Locate(orindevice)
	if err != nil {
		klog.ErrorS(err, "no pod with such device list", "devices list", strings.Join(devicesIDs, ":"))
		return nil, err
	}
	pod, err := s.Sitter.GetPod(curr.Namespace, curr.Name)
	if err != nil {
		klog.ErrorS(err, "failed to get pod", "pod", curr)
		return nil, err
	}
	// get board id from pod annotation which written by scheduler extender
	boardID, ok := pod.Annotations[common.AnnotationPodBindToBoard]
	if !ok {
		klog.Errorf("annotation %s does not on pod %s", common.AnnotationPodBindToBoard, curr)
		return nil, fmt.Errorf("annotation %s does not on pod %s", common.AnnotationPodBindToBoard, curr)
	}
	boardIDInt, err := strconv.ParseInt(boardID, 10, 0)
	if err != nil {
		klog.ErrorS(err, "parse board ID error", "boardID", boardID)
		return nil, err
	}
	orins := manager.BuildRequestOrinSet(pod)
	if orins.Len() == 0 {
		klog.V(4).InfoS("find empty orin request pod", "pod", curr)
		return &v1beta1.PreStartContainerResponse{}, nil
	}
	// get orin attr from provider
	attrs := s.DeviceProvider.GetOrinAttrs(int(boardIDInt), s.OrinID)
	if attrs == nil {
		klog.V(4).InfoS("find empty orin attr", "pod", curr, "board", boardID, "orin", s.OrinID)
		return &v1beta1.PreStartContainerResponse{}, nil
	}

	vpath := fmt.Sprintf("%s/%s/%s", HostVitualPath, s.ResourceName, devicesIDs[0])
	if err := populateOrinAttr(vpath, attrs); err != nil {
		klog.ErrorS(err, "populate Orin attr error", "vitual path", vpath, "attr", attrs)
		return nil, fmt.Errorf("populate Orin attr error")
	}

	return &v1beta1.PreStartContainerResponse{}, nil
}

func (s *OrinDeviceGrpcServer) GetPreferredAllocation(ctx context.Context, request *v1beta1.PreferredAllocationRequest) (*v1beta1.PreferredAllocationResponse, error) {
	return &v1beta1.PreferredAllocationResponse{}, nil
}

func (s *OrinDeviceGrpcServer) Allocate(ctx context.Context, request *v1beta1.AllocateRequest) (*v1beta1.AllocateResponse, error) {
	devicesIDs := []string{}
	for _, container := range request.ContainerRequests {
		devicesIDs = append(devicesIDs, container.DevicesIDs...)
	}
	if len(devicesIDs) == 0 {
		return &v1beta1.AllocateResponse{}, fmt.Errorf("devices is empty")
	}
	// make a vitual path, and device nums always 1
	mounts := []*v1beta1.Mount{
		{
			ContainerPath: fmt.Sprintf("/etc/%s", s.ResourceName),
			HostPath:      fmt.Sprintf("%s/%s/%s", HostVitualPath, s.ResourceName, devicesIDs[0]),
			ReadOnly:      true,
		},
	}

	return &v1beta1.AllocateResponse{
		ContainerResponses: []*v1beta1.ContainerAllocateResponse{{
			Mounts: mounts,
		}},
	}, nil

}

func patchNodeExtraResource(clientset *kubernetes.Clientset, provider provider.DeviceProvider, nodeName string) error {
	extraResource := make(map[v1.ResourceName]resource.Quantity)
	// board always large enough
	boardQ, _ := resource.ParseQuantity("1024")
	extraResource[v1.ResourceName(common.ExtendResouceTypeBoard)] = boardQ

	// build every board orin topology
	for _, bid := range provider.GetBoards() {
		resourceName := fmt.Sprintf("%s%d", common.ExtendResouceTypeBoardPrefix, bid)
		orinIDs := provider.GetBoardOrins(bid)
		resourceQ := manager.BuildDecimalMap(sets.NewInt(orinIDs...), 1)
		extraResource[v1.ResourceName(resourceName)] = *resource.NewQuantity(resourceQ, resource.DecimalSI)
	}

	nodeApply := applyv1.Node(nodeName).WithStatus(applyv1.NodeStatus().WithCapacity(v1.ResourceList(extraResource)))
	if _, err := clientset.CoreV1().Nodes().ApplyStatus(context.TODO(), nodeApply, metav1.ApplyOptions{FieldManager: "orin-device-plugin"}); err != nil {
		klog.ErrorS(err, "apple node extra resouce error", "resource", extraResource)
		return err
	}

	return nil
}

func populateOrinAttr(vpath string, attr map[string]interface{}) error {
	configPath := path.Join(vpath, "config.json")
	if err := os.MkdirAll(vpath, os.ModePerm); err != nil {
		return err
	}
	data, err := json.Marshal(attr)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, os.ModePerm)
}
