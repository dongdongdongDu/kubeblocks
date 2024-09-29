/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package apps

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// clusterComponentStatusTransformer transforms all cluster components' status.
type clusterComponentStatusTransformer struct{}

var _ graph.Transformer = &clusterComponentStatusTransformer{}

func (t *clusterComponentStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	// has no components defined
	if !transCtx.OrigCluster.IsStatusUpdating() {
		return nil
	}
	return t.reconcileComponentsStatus(transCtx)
}

func (t *clusterComponentStatusTransformer) reconcileComponentsStatus(transCtx *clusterTransformContext) error {
	cluster := transCtx.Cluster
	if cluster.Status.Components == nil {
		cluster.Status.Components = make(map[string]appsv1.ClusterComponentStatus)
	}
	for _, compSpec := range transCtx.ComponentSpecs {
		compKey := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      component.FullName(cluster.Name, compSpec.Name),
		}
		comp := &appsv1.Component{}
		if err := transCtx.Client.Get(transCtx.Context, compKey, comp); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		cluster.Status.Components[compSpec.Name] = t.buildClusterCompStatus(transCtx, comp, compSpec.Name)
	}

	// TODO: add sharding components to cluster status components here, which need to be refactored
	if err := t.buildShardingCompStatus(transCtx, cluster); err != nil {
		return err
	}

	return nil
}

func (t *clusterComponentStatusTransformer) buildShardingCompStatus(transCtx *clusterTransformContext, cluster *appsv1.Cluster) error {
	if len(cluster.Spec.ShardingSpecs) == 0 {
		return nil
	}
	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		shardingComps, err := component.ListShardingComponents(transCtx.Context, transCtx.Client, cluster, shardingSpec.Name)
		if err != nil {
			return err
		}
		if len(shardingComps) == 0 && shardingSpec.Shards != 0 {
			return fmt.Errorf("sharding components not found for sharding spec %s when sync cluster component status", shardingSpec.Name)
		}
		for _, shardingComp := range shardingComps {
			cluster.Status.Components[shardingComp.Name] = t.buildClusterCompStatus(transCtx, &shardingComp, shardingComp.Name)
		}
	}
	return nil
}

// buildClusterCompStatus builds cluster component status from specified component object.
func (t *clusterComponentStatusTransformer) buildClusterCompStatus(transCtx *clusterTransformContext,
	comp *appsv1.Component, compName string) appsv1.ClusterComponentStatus {
	var (
		cluster = transCtx.Cluster
		status  = cluster.Status.Components[compName]
	)

	phase := status.Phase
	t.updateClusterComponentStatus(comp, &status)

	if phase != status.Phase {
		phaseTransitionMsg := clusterComponentPhaseTransitionMsg(status.Phase)
		if transCtx.GetRecorder() != nil && phaseTransitionMsg != "" {
			transCtx.GetRecorder().Eventf(transCtx.Cluster, corev1.EventTypeNormal, componentPhaseTransition, phaseTransitionMsg)
		}
		transCtx.GetLogger().Info(fmt.Sprintf("cluster component phase transition: %s -> %s (%s)",
			phase, status.Phase, phaseTransitionMsg))
	}

	return status
}

// updateClusterComponentStatus sets the cluster component phase and messages conditionally.
func (t *clusterComponentStatusTransformer) updateClusterComponentStatus(comp *appsv1.Component,
	status *appsv1.ClusterComponentStatus) {
	if string(status.Phase) != string(comp.Status.Phase) {
		status.Phase = comp.Status.Phase
		if status.Message == nil {
			status.Message = comp.Status.Message
		} else {
			for k, v := range comp.Status.Message {
				status.Message[k] = v
			}
		}
	}
	// TODO(v1.0): status
	//// if ready flag not changed, don't update the ready time
	// ready := t.isClusterComponentPodsReady(comp.Status.Phase)
	// if status.PodsReady == nil || *status.PodsReady != ready {
	//	status.PodsReady = &ready
	//	if ready {
	//		now := metav1.Now()
	//		status.PodsReadyTime = &now
	//	}
	// }
}

// func (t *clusterComponentStatusTransformer) isClusterComponentPodsReady(phase appsv1.ClusterComponentPhase) bool {
//	podsReadyPhases := []appsv1.ClusterComponentPhase{
//		appsv1.RunningClusterCompPhase,
//		appsv1.StoppingClusterCompPhase,
//		appsv1.StoppedClusterCompPhase,
//	}
//	return slices.Contains(podsReadyPhases, phase)
// }

func clusterComponentPhaseTransitionMsg(phase appsv1.ClusterComponentPhase) string {
	if len(phase) == 0 {
		return ""
	}
	return fmt.Sprintf("component is %s", phase)
}
