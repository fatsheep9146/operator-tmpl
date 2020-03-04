package tracer

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const (
	defaultResyncPeriod  = time.Hour
	maxRetries           = 2
	catchTraceAnnotation = "catch-trace"
)

type Operator struct {
	client   *kubernetes.Clientset
	queue    workqueue.RateLimitingInterface
	podInf   cache.SharedIndexInformer
	podStore cache.Store
	recorder record.EventRecorder

	pods map[string]string
}

func New(ctx context.Context, cli *kubernetes.Clientset) *Operator {
	o := &Operator{
		queue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod"),
		client: cli,
	}
	client := o.client
	// start pod informer
	o.podInf = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return client.CoreV1().Pods("").List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return client.CoreV1().Pods("").Watch(options)
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

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})
	o.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "test-operator"})

	o.pods = make(map[string]string)

	go o.podInf.Run(ctx.Done())

	return o
}

func (o *Operator) Run(ctx context.Context) error {

	defer o.queue.ShutDown()

	if !cache.WaitForCacheSync(ctx.Done(), o.podInf.HasSynced) {
		klog.Info("msg", "pod informer unable to sync cache")
		return nil
	}

	for i := 0; i < 1; i++ {
		go wait.Until(o.worker, time.Second, ctx.Done())
	}
	// Block until the target provider is explicitly canceled.
	<-ctx.Done()
	return nil
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

	if err := o.syncHandler(key.(string)); err != nil {
		if o.queue.NumRequeues(key) < maxRetries {
			klog.V(2).Infof("Error syncing deployment %v: %v", key, err)
			o.queue.AddRateLimited(key)
			return true
		}
	}

	return true
}

func (o *Operator) syncHandler(key string) error {

	obj, _, err := o.podStore.GetByKey(key)
	if err != nil {
		return err
	}
	pod, isPod := obj.(*corev1.Pod)
	if !isPod {
		klog.Infof("got key %s which is not pod", key)
		return nil
	}
	klog.Infof("got pod %s added", key)
	if _, exist := pod.Annotations[catchTraceAnnotation]; !exist {
		return nil
	}

	klog.Infof("got pod %s which needed to be traced", key)
	if _, exist := o.pods[key]; exist {
		klog.Infof("pod %s is handled already", key)
		return nil
	}
	o.pods[key] = key

	// event.RecordPodEvent(o.recorder, pod, event.ActionStart, corev1.EventTypeNormal, "PodAction", "operator starts executing the PodAction for pod [%s]", key)

	// klog.Info("PodAction is being excuted")
	// time.Sleep(500 * time.Millisecond)

	// event.RecordPodEvent(o.recorder, pod, event.ActionEnd, corev1.EventTypeNormal, "PodAction", "operator have done the PodAction for pod [%s]", key)

	return nil
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
	klog.Infof("[Add] obj %v", key)
	o.enqueue(obj)
}

func (o *Operator) handlePodDelete(obj interface{}) {
	// create delete-pod tracer
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	klog.Infof("[Del] obj %v", key)
	o.enqueue(obj)
}

func (o *Operator) handlePodUpdate(oldo, curo interface{}) {
	// todo check for pod is schedulered
	// old := oldo.(*appsv1.StatefulSet)
	// cur := curo.(*appsv1.StatefulSet)
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(curo)
	klog.Infof("[Upd] obj %v", key)
	// o.enqueue(curo)
}
