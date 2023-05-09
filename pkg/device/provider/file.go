package provider

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/sets"
)

const FileDeviceProviderName = "file"

type OrinFileDeviceFactory struct{}

func (f *OrinFileDeviceFactory) Create(config string) (DeviceProvider, error) {
	return NewFileDeviceProvider(config)
}

type OrinFileDevice struct {
	NucIP        string    `yaml:"nuc_ip"`
	BoardDevices []*Device `yaml:"device"`
}

type Device struct {
	ID          int        `yaml:"id"`
	DeviceNum   string     `yaml:"device_num"`
	DeviceType  string     `yaml:"device_type"`
	ClusterName string     `yaml:"cluster_name"`
	Lidar       bool       `yaml:"lidar"`
	Camera      string     `yaml:"camera"`
	OrinSocs    []*OrinSoc `yaml:"socs"`
}

type OrinSoc struct {
	ID   int    `yaml:"id"`
	Name string `yaml:"name"`
	IP   string `yaml:"ip"`
}

type FileDeviceProvider struct {
	FilePath   string
	FileDevice *OrinFileDevice
}

func NewFileDeviceProvider(filePath string) (*FileDeviceProvider, error) {
	fod := new(OrinFileDevice)

	yamlData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(yamlData, fod); err != nil {
		return nil, err
	}
	return &FileDeviceProvider{FilePath: filePath, FileDevice: fod}, nil
}

func (fp *FileDeviceProvider) Name() string {
	return FileDeviceProviderName
}

func (fp *FileDeviceProvider) GetOrinClasses() map[int]sets.Int {
	res := make(map[int]sets.Int, 4)
	for _, b := range fp.FileDevice.BoardDevices {
		for _, soc := range b.OrinSocs {
			if boardIDs, ok := res[soc.ID]; !ok {
				res[soc.ID] = sets.NewInt(b.ID)
			} else {
				boardIDs.Insert(b.ID)
			}
		}
	}
	return res
}
func (fp *FileDeviceProvider) GetBoardAttrs(boardID int) map[string]interface{} {
	res := make(map[string]interface{}, 4)

	for _, b := range fp.FileDevice.BoardDevices {
		if b.ID == boardID {
			res[AttrKeyBoardDeviceNum] = b.DeviceNum
			res[AttrKeyBoardDeviceType] = b.DeviceType
			res[AttrKeyBoardClusterName] = b.ClusterName
			res[AttrKeyBoardLidar] = b.Lidar
			res[AttrKeyBoardCamera] = b.Camera
		}
	}
	return res
}
func (fp *FileDeviceProvider) GetOrinAttrs(boardID, OrinID int) map[string]interface{} {
	res := make(map[string]interface{}, 4)
	for _, b := range fp.FileDevice.BoardDevices {
		if b.ID == boardID {
			for _, s := range b.OrinSocs {
				if s.ID == OrinID {
					res[AttrKeyOrinIp] = s.IP
					res[AttrKeyOrinName] = s.Name
				}
			}
		}
	}
	return res
}

func (fp *FileDeviceProvider) GetBoards() []int {
	res := make([]int, len(fp.FileDevice.BoardDevices))
	for i, b := range fp.FileDevice.BoardDevices {
		res[i] = b.ID
	}
	return res
}
func (fp *FileDeviceProvider) GetBoardOrins(boardID int) []int {
	res := make([]int, 0, 4)
	for _, b := range fp.FileDevice.BoardDevices {
		if b.ID == boardID {
			for _, s := range b.OrinSocs {
				res = append(res, s.ID)
			}
		}
	}
	return res
}
