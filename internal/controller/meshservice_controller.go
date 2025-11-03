/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	meshv1alpha1 "github.com/mdarin/istio-integrity-operator/api/v1alpha1"
	"github.com/mdarin/istio-integrity-operator/internal/integrity"
)

// MeshServiceReconciler reconciles a MeshService object
type MeshServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mesh.istio.operator,resources=meshservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mesh.istio.operator,resources=meshservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mesh.istio.operator,resources=meshservices/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=gateways,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MeshServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Starting reconciliation", "namespacedName", req.NamespacedName)

	// 1. Get the MeshService instance
	meshService, err := r.getMeshService(ctx, req.NamespacedName)
	if err != nil {
		log.Error(err, "unable to fetch MeshService")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Update status to Checking
	if err := r.updateStatus(ctx, meshService, meshv1alpha1.Checking, nil, nil); err != nil {
		return ctrl.Result{}, err
	}

	// 3. Create integrity operator
	operator := integrity.NewSQLiteIntegrityOperator(r.Client)

	// 4. Build relational model from cluster state
	model, err := operator.BuildRelationalModel(ctx)
	if err != nil {
		log.Error(err, "failed to build relational model")
		return ctrl.Result{}, r.updateStatusWithError(ctx, meshService, err)
	}

	// 5. Create in-memory SQLite database
	db, err := operator.CreateInMemoryDB(model)
	if err != nil {
		log.Error(err, "failed to create in-memory database")
		return ctrl.Result{}, r.updateStatusWithError(ctx, meshService, err)
	}
	defer db.Close()

	// 6. Check referential integrity
	report, err := operator.CheckIntegrity(db)
	if err != nil {
		log.Error(err, "failed to check integrity")
		return ctrl.Result{}, r.updateStatusWithError(ctx, meshService, err)
	}

	// 7. Compute repair plans if inconsistent
	var repairActions []meshv1alpha1.RepairAction
	if !report.IsConsistent {
		repairActions, err = operator.ComputeRepairPlans(db, report)
		if err != nil {
			log.Error(err, "failed to compute repair plans")
			return ctrl.Result{}, r.updateStatusWithError(ctx, meshService, err)
		}
	}

	// 8. Update status based on results
	state := meshv1alpha1.Consistent
	if !report.IsConsistent {
		if len(repairActions) > 0 {
			state = meshv1alpha1.RepairPending
		} else {
			state = meshv1alpha1.Inconsistent
		}
	}

	if err := r.updateStatus(ctx, meshService, state, report.Violations, repairActions); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Reconciliation completed",
		"consistent", report.IsConsistent,
		"violations", len(report.Violations),
		"repairs", len(repairActions))

	return ctrl.Result{}, nil
}

func (r *MeshServiceReconciler) getMeshService(ctx context.Context, namespacedName client.ObjectKey) (*meshv1alpha1.MeshService, error) {
	var meshService meshv1alpha1.MeshService
	if err := r.Get(ctx, namespacedName, &meshService); err != nil {
		return nil, err
	}
	return &meshService, nil
}

func (r *MeshServiceReconciler) updateStatus(
	ctx context.Context,
	meshService *meshv1alpha1.MeshService,
	state meshv1alpha1.ConsistencyState,
	violations []meshv1alpha1.ConstraintViolation,
	repairActions []meshv1alpha1.RepairAction,
) error {
	meshService.Status.ConsistencyState = state
	meshService.Status.Violations = violations
	meshService.Status.RepairActions = repairActions

	if err := r.Status().Update(ctx, meshService); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}
	return nil
}

func (r *MeshServiceReconciler) updateStatusWithError(ctx context.Context, meshService *meshv1alpha1.MeshService, err error) error {
	// Update status with error information
	meshService.Status.ConsistencyState = meshv1alpha1.Inconsistent
	meshService.Status.Violations = []meshv1alpha1.ConstraintViolation{
		{
			Type:     "ReconciliationError",
			Resource: meshService.Name,
			Message:  err.Error(),
			Severity: "Error",
		},
	}
	return r.Status().Update(ctx, meshService)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MeshServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&meshv1alpha1.MeshService{}).
		Named("meshservice").
		Complete(r)
}
