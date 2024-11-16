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

package util

import (
	"context"
	"fmt"
	"hash/fnv"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctlruntime "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	maxRetryAttempts = 30
	retryInterval    = 10 * time.Second
)

func SendEventWithMessage(logger *logr.Logger, reason string, message string) {
	go func() {
		err := createOrUpdateEvent(reason, message)
		if logger != nil && err != nil {
			logger.Error(err, "failed to send event",
				"reason", reason,
				"message", message)
		}
	}()
}

func newEvent(reason string, message string) *corev1.Event {
	now := metav1.Now()
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateEventName(reason, message),
			Namespace: namespace(),
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: namespace(),
			Name:      podName(),
			UID:       types.UID(podUID()),
			FieldPath: proto.ProbeEventFieldPath,
		},
		Reason:  reason,
		Message: message,
		Source: corev1.EventSource{
			Component: proto.ProbeEventSourceComponent,
			Host:      nodeName(),
		},
		FirstTimestamp:      now,
		LastTimestamp:       now,
		EventTime:           metav1.NowMicro(),
		ReportingController: proto.ProbeEventReportingController,
		ReportingInstance:   podName(),
		Action:              reason,
		Type:                "Normal",
		Count:               1,
	}
}

func createOrUpdateEvent(reason, message string) error {
	clientSet, err := getK8sClientSet()
	if err != nil {
		return err
	}
	eventsClient := clientSet.CoreV1().Events(namespace())
	eventName := generateEventName(reason, message)

	var event *corev1.Event
	for i := 0; i < maxRetryAttempts; i++ {
		event, err = eventsClient.Get(context.Background(), eventName, metav1.GetOptions{})
		if err == nil || k8serrors.IsNotFound(err) {
			break
		}
		time.Sleep(retryInterval)
	}

	if err == nil && event != nil {
		// update
		event.Count++
		event.LastTimestamp = metav1.Now()
		for i := 0; i < maxRetryAttempts; i++ {
			_, err = eventsClient.Update(context.Background(), event, metav1.UpdateOptions{})
			if err == nil {
				return nil
			}
			time.Sleep(retryInterval)
		}
		return errors.Wrapf(err, "failed to update event %s after %d attempts", eventName, maxRetryAttempts)
	}

	if k8serrors.IsNotFound(err) {
		// create
		event = newEvent(reason, message)
		for i := 0; i < maxRetryAttempts; i++ {
			_, err = eventsClient.Create(context.Background(), event, metav1.CreateOptions{})
			if err == nil {
				return nil
			}
			time.Sleep(retryInterval)
		}
		return errors.Wrapf(err, "failed to create event %s after %d attempts", eventName, maxRetryAttempts)
	}
	return errors.Wrapf(err, "failed to get event %s after %d attempts", eventName, maxRetryAttempts)
}

func getK8sClientSet() (*kubernetes.Clientset, error) {
	restConfig, err := ctlruntime.GetConfig()
	if err != nil {
		return nil, errors.Wrap(err, "get kubeConfig failed")
	}
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}

func generateEventName(reason, message string) string {
	hash := fnv.New32a()
	hash.Write([]byte(fmt.Sprintf("%s.%s.%s", podName(), reason, message)))
	return fmt.Sprintf("%s.%x", podName(), hash.Sum32())
}
