package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/superedge/orin-device-system/pkg/device/kubeapis"
	"github.com/superedge/orin-device-system/pkg/device/plugin"
	"github.com/superedge/orin-device-system/pkg/device/provider"
)

var (
	kubeconfig           string
	nodeName             string
	deviceProvider       string
	deviceProviderConfig string
)

func InitFlag() {
	flag.StringVar(&nodeName, "node-name", "", "node name")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "kubeconfig path")
	flag.StringVar(&deviceProvider, "provider", "file", "device provider current support 'file'")
	flag.StringVar(&deviceProviderConfig, "provider-config", "", "device provider config file path")

}

func BuildClientSet(kubeconfigPath string) (*kubernetes.Clientset, error) {
	var restconfig *rest.Config
	var err error
	if kubeconfigPath == "" {
		restconfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else {
		restconfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(restconfig)
	if err != nil {
		klog.Fatalf("Failed to init clientset due to %v", err)
	}

	return clientset, nil
}

func ExitSignal() <-chan os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT)
	return ch
}

func main() {
	InitFlag()
	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()

	clientSet, err := BuildClientSet(kubeconfig)
	if err != nil {
		klog.Fatal(err.Error())
		return
	}
	sitter := kubeapis.NewSitter(clientSet, nodeName)
	go sitter.Start()

	if ok := cache.WaitForCacheSync(
		make(chan struct{}),
		sitter.HasSynced,
	); !ok {
		klog.Fatal("failed to wait for caches to sync")
	}

	providerfactory, ok := provider.ProviderMap[deviceProvider]
	if !ok {
		klog.Fatalln("invalid device provider")
		return
	}
	p, err := providerfactory.Create(deviceProviderConfig)
	if err != nil {
		klog.Fatalln("failed to create device provider")
		return
	}
	odc := &plugin.OrinDeviceConfig{
		Sitter:         sitter,
		DeviceProvider: p,
		DeviceLocator:  make(map[v1.ResourceName]kubeapis.DeviceLocator),
		NodeName:       nodeName,
		ClientSet:      clientSet,
	}
	plug, err := plugin.NewOrinDevicePlugin(odc)
	if err != nil {
		klog.Fatalln(err.Error())
		return
	}
	plug.Run(make(chan struct{}))
	klog.Info("start to run orin device plugin")
	<-ExitSignal()
}
