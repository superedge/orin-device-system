package provider

import "k8s.io/apimachinery/pkg/util/sets"

const (
	AttrKeyBoardDeviceNum   = "device_num"
	AttrKeyBoardDeviceType  = "device_type"
	AttrKeyBoardClusterName = "cluster_name"
	AttrKeyBoardLidar       = "lidar"
	AttrKeyBoardCamera      = "camera"

	AttrKeyOrinIp   = "ip"
	AttrKeyOrinName = "name"
)

var ProviderMap = map[string]DeviceProviderFactory{FileDeviceProviderName: &OrinFileDeviceFactory{}}

type DeviceProviderFactory interface {
	Create(config string) (DeviceProvider, error)
}

type BoardOrinIndex struct {
	BoardID int
	OrinID  int
}

type DeviceProvider interface {
	Name() string

	GetOrinClasses() map[int]sets.Int
	GetBoardAttrs(boardID int) map[string]interface{}
	GetOrinAttrs(boardID, OrinID int) map[string]interface{}
	GetBoards() []int
	GetBoardOrins(boardID int) []int
}
