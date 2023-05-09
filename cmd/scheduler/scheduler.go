package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/julienschmidt/httprouter"
	"github.com/superedge/orin-device-system/pkg/scheduler/manager"
	"github.com/superedge/orin-device-system/pkg/scheduler/routes"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const (
	versionPath = "/version"
)

var (
	kubeconfig string
	port       int
	threadness int
)

var (
	Version              string // injected via ldflags at build time
	onlyOneSignalHandler = make(chan struct{})

	shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
)

func InitFlag() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig")
	flag.IntVar(&port, "port", 80, "port to orin extend scheduler")
	flag.IntVar(&threadness, "threadness", 4, "thread for cache controller")
}

func main() {
	InitFlag()
	klog.InitFlags(nil)
	flag.Parse()

	// build kubernetes clientset
	clientset, err := BuildClientSet(kubeconfig)
	if err != nil {
		klog.Fatalf("failed to init kube client: %v", err)
	}

	// build route
	router := httprouter.New()
	AddVersion(router)

	scache := manager.NewScheduleCache()

	mng := manager.NewManager(scache, clientset)

	routes.AddPredicate(router, routes.NewPredicate("orin-system", mng))

	routes.AddPrioritize(router, routes.NewPrioritize("orin-system", mng))

	routes.AddBind(router, routes.NewBind("orin-system", mng))

	// build controller
	stopCh := SetupSignalHandler()

	controller, err := manager.NewController(clientset, mng, stopCh)
	if err != nil {
		klog.Fatal(err)
	}
	go controller.Run(threadness, stopCh)

	klog.Infof("info: server starting on the port :%d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), router); err != nil {
		klog.Fatal(err)
	}
}
func BuildClientSet(kubeconfigPath string) (kubernetes.Interface, error) {
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

// SetupSignalHandler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func SetupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}
func VersionRoute(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, fmt.Sprint(Version))
}

func AddVersion(router *httprouter.Router) {
	router.GET(versionPath, routes.DebugLogging(VersionRoute, versionPath))
}
