package tracer

import (
	"context"
	"time"

	"github.com/prometheus/prometheus/discovery/targetgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	log "k8s.io/klog"
)

const (
	defaultResyncPeriod = time.Hour
	maxRetries          = 2
)

type Operator struct {
	client   *kubernetes.Clientset
	queue    workqueue.RateLimitingInterface
	podInf   cache.SharedIndexInformer
	podStore cache.Store
}

func New(ctx context.Context, cli *kubernetes.Clientset) *Operator {
	o := &Operator{
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod"),
	}

	// start pod informer
	o.podInf = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return cli.CoreV1().Pods("").List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return cli.CoreV1().Pods("").Watch(options)
			},
		},
		&corev1.Pod{},
		defaultResyncPeriod,
		cache.Indexers{},
	)

	o.podStore = o.podInf.GetStore()

	o.podInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    o.handlePodAdd,
		DeleteFunc: o.handlePodDelete,
		UpdateFunc: o.handlePodUpdate,
	})

	go o.podInf.Run(ctx.Done())

	return o
}

func (o *Operator) Run(ctx context.Context) {

	defer o.queue.ShutDown()

	if !cache.WaitForCacheSync(ctx.Done(), o.podInf.HasSynced) {
		log.Info("msg", "pod informer unable to sync cache")
		return
	}

	for i := 0; i < 1; i++ {
		go wait.Until(o.worker, time.Second, ctx.Done())
	}
	// Block until the target provider is explicitly canceled.
	<-ctx.Done()
	return
}

func (o *Operator) worker() {
	for o.processNextWorkItem() {
	}
}

func (o *Operator) processNextWorkItem() bool {
	key, quit := o.queue.Get()
	if quit {
		return false
	}
	defer o.queue.Done(key)

	if err := dc.syncHandler(key.(string)); err != nil {
		if o.queue.NumRequeues(key) < maxRetries {
			log.V(2).Infof("Error syncing deployment %v: %v", key, err)
			o.queue.AddRateLimited(key)
			return true
		}
	}

	return true
}

func (o *Operator) syncHandler(key string) {
	keyObj, quit := o.queue.Get()
	if quit {
		return false
	}
	defer p.queue.Done(keyObj)
	key := keyObj.(string)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return true
	}

	o, exists, err := p.store.GetByKey(key)
	if err != nil {
		return true
	}
	if !exists {
		send(ctx, p.logger, RolePod, ch, &targetgroup.Group{Source: podSourceFromNamespaceAndName(namespace, name)})
		return true
	}
	eps, err := convertToPod(o)
	if err != nil {
		level.Error(p.logger).Log("msg", "converting to Pod object failed", "err", err)
		return true
	}
	send(ctx, p.logger, RolePod, ch, p.buildPod(eps))
	return true
}

func (o *Operator) enqueue(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}

	o.queue.Add(key)
}

func (o *Operator) handlePodAdd(obj interface{}) {
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	log.Info("[Add] obj %v", key)
	o.enqueue(obj)
}

func (o *Operator) handlePodDelete(obj interface{}) {
	// create delete-pod tracer
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	log.Info("[Del] obj %v", key)
	o.enqueue(obj)
}

func (o *Operator) handlePodUpdate(oldo, curo interface{}) {
	// todo check for pod is schedulered
	// old := oldo.(*appsv1.StatefulSet)
	// cur := curo.(*appsv1.StatefulSet)
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(curo)
	log.Info("[Upd] obj %v", key)
	o.enqueue(curo)
}
