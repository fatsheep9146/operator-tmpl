package main

import (
	"context"
	"flag"
	"time"

	operator "github.com/fatsheep9146/operator-tmpl/pkg"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

func main() {
	flag.Set("alsologtostderr", "true")
	flag.Parse()

	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)
	// Sync the glog and klog flags.
	flag.CommandLine.VisitAll(func(f1 *flag.Flag) {
		f2 := klogFlags.Lookup(f1.Name)
		if f2 != nil {
			value := f1.Value.String()
			f2.Value.Set(value)
		}
	})
	go wait.Forever(klog.Flush, 5*time.Second)
	defer klog.Flush()

	klog.Info("start operator")

	cfg, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	ctx := context.TODO()

	op := operator.New(ctx, client)
	if err := op.Run(ctx); err != nil {
		panic(err)
	}
}
