package reconciler

import (
	"context"
	"log"
	"path/filepath"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/sync"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	pipelineruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/pipelinerun"
	pipelinerunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/pipelinerun"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
)

func NewController() func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		run := params.New()
		err := run.Clients.NewClients(ctx, &run.Info)
		if err != nil {
			log.Fatal("failed to init clients : ", err)
		}

		kinteract, err := kubeinteraction.NewKubernetesInteraction(run)
		if err != nil {
			log.Fatal("failed to init kinit client : ", err)
		}

		go func() {
			if err := run.WatchConfigMapChanges(ctx, run); err != nil {
				log.Fatal(err)
			}
		}()

		run.Info.Pac.LogURL = run.Clients.ConsoleUI.URL()

		pipelineRunInformer := pipelineruninformer.Get(ctx)

		c := &Reconciler{
			run:               run,
			kinteract:         kinteract,
			pipelineRunLister: pipelineRunInformer.Lister(),
			qm:                sync.NewQueueManager(run.Clients.Log),
		}
		impl := pipelinerunreconciler.NewImpl(ctx, c, ctrlOpts())

		if err := c.qm.InitQueues(ctx, run.Clients.Tekton, run.Clients.PipelineAsCode); err != nil {
			log.Fatal("failed to init queues", err)
		}

		pipelineRunInformer.Informer().AddEventHandler(controller.HandleAll(checkStateAndEnqueue(impl)))

		return impl
	}
}

// enqueue only the pipelineruns which are in `started` state
// pipelinerun will have a label `pipelinesascode.tekton.dev/state` to describe the state
func checkStateAndEnqueue(impl *controller.Impl) func(obj interface{}) {
	return func(obj interface{}) {
		object, err := kmeta.DeletionHandlingAccessor(obj)
		if err == nil {
			_, exist := object.GetLabels()[filepath.Join(pipelinesascode.GroupName, "state")]
			if exist {
				impl.EnqueueKey(types.NamespacedName{Namespace: object.GetNamespace(), Name: object.GetName()})
			}
		}
	}
}

func ctrlOpts() func(impl *controller.Impl) controller.Options {
	return func(impl *controller.Impl) controller.Options {
		return controller.Options{
			FinalizerName: pipelinesascode.GroupName,
			PromoteFilterFunc: func(obj interface{}) bool {
				_, exist := obj.(*v1beta1.PipelineRun).GetLabels()[filepath.Join(pipelinesascode.GroupName, "state")]
				return exist
			},
		}
	}
}
