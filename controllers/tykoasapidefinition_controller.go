/*


Licensed under the Mozilla Public License (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.mozilla.org/en-US/MPL/2.0/

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	util "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/TykTechnologies/tyk-operator/api/v1alpha1"
	"github.com/TykTechnologies/tyk-operator/pkg/client/klient"
	"github.com/TykTechnologies/tyk-operator/pkg/environment"
	"github.com/TykTechnologies/tyk-operator/pkg/keys"
	"github.com/buger/jsonparser"
	"github.com/go-logr/logr"
)

const (
	TykOASConfigMapKey  = "spec.tykOAS.configmapRef.name"
	TykOASExtenstionStr = "x-tyk-api-gateway"
)

// TykOasApiDefinitionReconciler reconciles a TykOasApiDefinition object
type TykOasApiDefinitionReconciler struct {
	client.Client
	Log    logr.Logger
	Env    environment.Env
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tyk.tyk.io,resources=tykoasapidefinitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tyk.tyk.io,resources=tykoasapidefinitions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tyk.tyk.io,resources=tykoasapidefinitions/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *TykOasApiDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("TykOasApiDefinition", req.NamespacedName.String())

	log.Info("Reconciling Tyk OAS instance")

	var reqA time.Duration
	var apiID string
	var markForDeletion bool

	// Lookup Tyk OAS object
	tykOAS := &v1alpha1.TykOasApiDefinition{}
	if err := r.Get(ctx, req.NamespacedName, tykOAS); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	_, ctx, err := HttpContext(ctx, r.Client, &r.Env, tykOAS, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	_, err = util.CreateOrUpdate(ctx, r.Client, tykOAS, func() error {
		if !tykOAS.ObjectMeta.DeletionTimestamp.IsZero() {
			markForDeletion = true

			return delete(ctx, tykOAS)
		}

		util.AddFinalizer(tykOAS, keys.TykOASFinalizerName)

		apiID, err = r.createOrUpdateTykOASAPI(ctx, tykOAS, log)
		if err != nil {
			log.Error(err, "Failed to create/update Tyk OAS API")
			return err
		}
		return nil
	})

	if !markForDeletion {
		var transactionInfo v1alpha1.TransactionInfo

		if err == nil {
			transactionInfo.Status = v1alpha1.Successful
			transactionInfo.Time = metav1.Now()
		} else {
			reqA = queueAfter

			transactionInfo.Status = v1alpha1.Failed
			transactionInfo.Time = metav1.Now()
			transactionInfo.Error = err.Error()
		}

		tykOAS.Status.LatestTransaction = transactionInfo

		if err = r.updateStatus(ctx, tykOAS, apiID, log); err != nil {
			log.Error(err, "Failed to update status of Tyk OAS CRD")
			return ctrl.Result{RequeueAfter: reqA}, err
		}
	}

	if err := klient.Universal.HotReload(ctx); err != nil {
		log.Error(err, "Failed to reload gateway")
		return ctrl.Result{RequeueAfter: reqA}, err
	}

	log.Info("Completed reconciling Tyk OAS instance")

	return ctrl.Result{RequeueAfter: reqA}, err
}

func (r *TykOasApiDefinitionReconciler) createOrUpdateTykOASAPI(ctx context.Context,
	tykOASCrd *v1alpha1.TykOasApiDefinition, log logr.Logger) (string, error,
) {
	var cm v1.ConfigMap

	ns := types.NamespacedName{
		Name:      tykOASCrd.Spec.TykOAS.ConfigmapRef.Name,
		Namespace: tykOASCrd.Spec.TykOAS.ConfigmapRef.Namespace,
	}

	err := r.Client.Get(ctx, ns, &cm)
	if err != nil {
		log.Error(err, "Failed to fetch config map")
		return "", err
	}

	tykOASDoc := cm.Data[tykOASCrd.Spec.TykOAS.ConfigmapRef.KeyName]

	_, _, _, err = jsonparser.Get([]byte(tykOASDoc), TykOASExtenstionStr)
	if err != nil {
		errMsg := "invalid Tyk OAS APIDefinition. Failed to fetch value of `x-tyk-api-gateway` "
		log.Error(err, errMsg)

		return "", fmt.Errorf("%s: %s", errMsg, err.Error())
	}

	apiID, err := getAPIID(tykOASCrd, tykOASDoc)
	if err != nil {
		return "", err
	}

	if apiID == "" {
		apiID = EncodeNS(client.ObjectKeyFromObject(tykOASCrd).String())
	}

	exists := klient.Universal.TykOAS().Exists(ctx, apiID)
	if !exists {
		if err = klient.Universal.TykOAS().Create(ctx, apiID, tykOASDoc); err != nil {
			log.Error(err, "Failed to create Tyk OAS API")
			return "", err
		}
	} else {
		if err = klient.Universal.TykOAS().Update(ctx, apiID, tykOASDoc); err != nil {
			log.Error(err, "Failed to update Tyk OAS API")
			return "", err
		}
	}

	return apiID, nil
}

func (r *TykOasApiDefinitionReconciler) updateStatus(ctx context.Context, tykOASCrd *v1alpha1.TykOasApiDefinition,
	apiID string, log logr.Logger,
) error {
	var cm v1.ConfigMap

	log.Info("Updating status of Tyk OAS instance")

	if tykOASCrd.Status.ID == "" {
		tykOASCrd.Status.ID = apiID
	}

	cmNS := types.NamespacedName{
		Name:      tykOASCrd.Spec.TykOAS.ConfigmapRef.Name,
		Namespace: tykOASCrd.Spec.TykOAS.ConfigmapRef.Namespace,
	}

	err := r.Client.Get(ctx, cmNS, &cm)
	if err != nil {
		log.Error(err, "Failed to fetch config map")

		tykOASCrd.Status.LatestTransaction.Status = v1alpha1.Failed
		tykOASCrd.Status.LatestTransaction.Error = err.Error()
		tykOASCrd.Status.LatestTransaction.Time = metav1.Now()
	} else {
		tykOASDoc := cm.Data[tykOASCrd.Spec.TykOAS.ConfigmapRef.KeyName]

		state, err := jsonparser.GetBoolean([]byte(tykOASDoc), TykOASExtenstionStr, "info", "state", "active")
		// do not throw error if field doesn't exist
		if err != nil && err != jsonparser.KeyPathNotFoundError {
			log.Error(err, "Failed to fetch state information from Tyk OAS document")
		} else {
			tykOASCrd.Status.Enabled = state
		}

		str, err := jsonparser.GetString([]byte(tykOASDoc), TykOASExtenstionStr, "server", "customDomain")
		// do not throw error if field doesn't exist
		if err != nil && err != jsonparser.KeyPathNotFoundError {
			log.Error(err, "Failed to fetch domain information from Tyk OAS document")
		} else {
			tykOASCrd.Status.Domain = str
		}

		str, err = jsonparser.GetString([]byte(tykOASDoc), TykOASExtenstionStr, "server", "listenPath", "value")
		// do not throw error if field doesn't exist
		if err != nil && err != jsonparser.KeyPathNotFoundError {
			log.Error(err, "Failed to fetch listen path information from Tyk OAS document")
		} else {
			tykOASCrd.Status.ListenPath = str
		}

		str, err = jsonparser.GetString([]byte(tykOASDoc), TykOASExtenstionStr, "upstream", "url")
		// do not throw error if field doesn't exist
		if err != nil && err != jsonparser.KeyPathNotFoundError {
			log.Error(err, "Failed to fetch upstream url  information from Tyk OAS document")
		} else {
			tykOASCrd.Status.TargetURL = str
		}
	}

	return r.Client.Status().Update(ctx, tykOASCrd)
}

func getAPIID(tykOASCrd *v1alpha1.TykOasApiDefinition, tykOASDoc string) (string, error) {
	if tykOASCrd.Status.ID != "" {
		return tykOASCrd.Status.ID, nil
	}

	val, err := jsonparser.GetString([]byte(tykOASDoc), TykOASExtenstionStr, "info", "id")
	// do not throw error if id doesn't exist
	if err != nil && err != jsonparser.KeyPathNotFoundError {
		return "", err
	}

	return val, nil
}

func delete(ctx context.Context, tykOASCrd *v1alpha1.TykOasApiDefinition) error {
	if tykOASCrd.Status.ID != "" {
		if err := klient.Universal.TykOAS().Delete(ctx, tykOASCrd.Status.ID); err != nil {
			return err
		}
	}

	util.RemoveFinalizer(tykOASCrd, keys.TykOASFinalizerName)

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TykOasApiDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(),
		&v1alpha1.TykOasApiDefinition{},
		TykOASConfigMapKey,
		func(o client.Object) []string {
			tykOAS := o.(*v1alpha1.TykOasApiDefinition) //nolint:errcheck
			return []string{tykOAS.Spec.TykOAS.ConfigmapRef.Name}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.TykOasApiDefinition{}).
		Watches(
			&source.Kind{Type: &v1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(r.findOASApisDependentOnConfigmap),
			builder.WithPredicates(r.configmapEvents()),
		).
		Complete(r)
}

func (r *TykOasApiDefinitionReconciler) findOASApisDependentOnConfigmap(cm client.Object) []reconcile.Request {
	tykOASAPIs := &v1alpha1.TykOasApiDefinitionList{}
	listOps := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(TykOASConfigMapKey, cm.GetName()),
	}

	if err := r.List(context.TODO(), tykOASAPIs, listOps); err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(tykOASAPIs.Items))
	for i, item := range tykOASAPIs.Items { //nolint
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}

	return requests
}

func (r *TykOasApiDefinitionReconciler) configmapEvents() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
	}
}
