package main

import (
	"context"

	operator "github.com/fatsheep9146/operator-tmpl/pkg"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	ctx := context.TODO()

	op := operator.New(ctx, client)
	op.Run()
}
