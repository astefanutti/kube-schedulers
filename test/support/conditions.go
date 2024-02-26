package support

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

type conditionType interface {
	~string
}

// TODO: to be replaced with a generic version once common struct fields of a type set can be used.
// See https://github.com/golang/go/issues/48522
func ConditionStatus[T conditionType](conditionType T) func(any) corev1.ConditionStatus {
	return func(object any) corev1.ConditionStatus {
		switch o := object.(type) {

		case *corev1.Node:
			if c := getNodeCondition(o.Status.Conditions, corev1.NodeConditionType(conditionType)); c != nil {
				return c.Status
			}
		case *batchv1.Job:
			if c := getJobCondition(o.Status.Conditions, batchv1.JobConditionType(conditionType)); c != nil {
				return c.Status
			}
		case *appsv1.Deployment:
			if c := getDeploymentCondition(o.Status.Conditions, appsv1.DeploymentConditionType(conditionType)); c != nil {
				return c.Status
			}
		}

		return corev1.ConditionUnknown
	}
}

func getNodeCondition(conditions []corev1.NodeCondition, conditionType corev1.NodeConditionType) *corev1.NodeCondition {
	for _, c := range conditions {
		if c.Type == conditionType {
			return &c
		}
	}
	return nil
}

func getJobCondition(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType) *batchv1.JobCondition {
	for _, c := range conditions {
		if c.Type == conditionType {
			return &c
		}
	}
	return nil
}

func getDeploymentCondition(conditions []appsv1.DeploymentCondition, conditionType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for _, c := range conditions {
		if c.Type == conditionType {
			return &c
		}
	}
	return nil
}
