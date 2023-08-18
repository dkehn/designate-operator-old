/*

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

package designatebackendbind9

import (
	designatev1beta1 "github.com/openstack-k8s-operators/designate-operator/api/v1beta1"
	designate "github.com/openstack-k8s-operators/designate-operator/pkg/designate"
	common "github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/affinity"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// ServiceCommand -
	ServiceCommand = "/usr/local/bin/kolla_set_configs && /usr/local/bin/kolla_start"
)

// Deployment func
func Deployment(
	instance *designatev1beta1.DesignateBackendbind9,
	configHash string,
	labels map[string]string,
	annotations map[string]string,
) *appsv1.Deployment {
	rootUser := int64(0)
	// Designate's uid and gid magic numbers come from the 'designate-user' in
	// https://github.com/openstack/kolla/blob/master/kolla/common/users.py
	// designateUser := int64(42411)
	// designateGroup := int64(42411)

	// livenessProbe := &corev1.Probe{
	// 	// TODO might need tuning
	// 	TimeoutSeconds:      5,
	// 	PeriodSeconds:       3,
	// 	InitialDelaySeconds: 3,
	// }
	// startupProbe := &corev1.Probe{
	// 	// TODO might need tuning
	// 	TimeoutSeconds:      5,
	// 	FailureThreshold:    12,
	// 	PeriodSeconds:       5,
	// 	InitialDelaySeconds: 5,
	// }
	args := []string{"-c"}
	// var probeCommand []string
	if instance.Spec.Debug.Service {
		args = append(args, common.DebugCommand)
		// livenessProbe.Exec = &corev1.ExecAction{
		// 	Command: []string{
		// 		"/bin/true",
		// 	},
		// }
		// startupProbe.Exec = livenessProbe.Exec
		// probeCommand = []string{
		// 	"/bin/sleep", "infinity",
		// }
	} else {
		args = append(args, ServiceCommand)
		// livenessProbe.HTTPGet = &corev1.HTTPGetAction{
		// 	Port: intstr.FromInt(8080),
		// }
		// startupProbe.HTTPGet = livenessProbe.HTTPGet
		// // Probe doesn't run kolla_set_configs because it uses the 'designate' uid
		// // and gid and doesn't have permissions to make files be owned by root,
		// // so designate.conf is in its original location
		// probeCommand = []string{
		// 	"/usr/local/bin/container-scripts/healthcheck.py",
		// 	"scheduler",
		// 	"/var/lib/config-data/merged/designate.conf",
		// }
	}

	envVars := map[string]env.Setter{}
	envVars["KOLLA_CONFIG_FILE"] = env.SetValue(KollaConfig)
	envVars["KOLLA_CONFIG_STRATEGY"] = env.SetValue("COPY_ALWAYS")
	envVars["CONFIG_HASH"] = env.SetValue(configHash)

	volumeMounts := designate.GetAllVolumeMounts()

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: instance.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
					Labels:      labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: instance.Spec.ServiceAccount,
					Volumes: designate.GetOpenstackVolumes(
						designate.GetServiceConfigConfigMapName(instance.Name),
					),
					Containers: []corev1.Container{
						{
							Name: designate.ServiceName + "-backendbind9",
							Command: []string{
								"/bin/sleep", "60000",
							},
							// Command: []string{
							// 	"/bin/bash",
							// },
							// Args:  args,
							Image: instance.Spec.ContainerImage,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &rootUser,
							},
							Env:          env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts: volumeMounts,
							Resources:    instance.Spec.Resources,
							// StartupProbe:  startupProbe,
							// LivenessProbe: livenessProbe,
						},
						// {
						// 	Name:    "probe",
						// 	Command: probeCommand,
						// 	Image:   instance.Spec.ContainerImage,
						// 	SecurityContext: &corev1.SecurityContext{
						// 		RunAsUser:  &designateUser,
						// 		RunAsGroup: &designateGroup,
						// 	},
						// 	VolumeMounts: volumeMounts,
						// },
					},
					NodeSelector: instance.Spec.NodeSelector,
				},
			},
		},
	}
	deployment.Spec.Template.Spec.Volumes = designate.GetVolumes(
		designate.GetOwningDesignateName(instance))

	// If possible two pods of the same service should not
	// run on the same worker node. If this is not possible
	// the get still created on the same worker node.
	deployment.Spec.Template.Spec.Affinity = affinity.DistributePods(
		common.AppSelector,
		[]string{
			designate.ServiceName,
		},
		corev1.LabelHostname,
	)
	if instance.Spec.NodeSelector != nil && len(instance.Spec.NodeSelector) > 0 {
		deployment.Spec.Template.Spec.NodeSelector = instance.Spec.NodeSelector
	}

	initContainerDetails := designate.APIDetails{
		ContainerImage:       instance.Spec.ContainerImage,
		DatabaseHost:         instance.Spec.DatabaseHostname,
		DatabaseUser:         instance.Spec.DatabaseUser,
		DatabaseName:         designate.DatabaseName,
		OSPSecret:            instance.Spec.Secret,
		TransportURLSecret:   instance.Spec.TransportURLSecret,
		DBPasswordSelector:   instance.Spec.PasswordSelectors.Database,
		UserPasswordSelector: instance.Spec.PasswordSelectors.Service,
		BackendType:          instance.Spec.BackendType,
		VolumeMounts:         designate.GetAllVolumeMounts(),
		Debug:                instance.Spec.Debug.InitContainer,
	}
	deployment.Spec.Template.Spec.InitContainers = designate.InitContainer(initContainerDetails)

	// TODO: Clean up this hack
	// Add custom config for the API Service
	envVars = map[string]env.Setter{}
	envVars["CustomConf"] = env.SetValue(common.CustomServiceConfigFileName)
	deployment.Spec.Template.Spec.InitContainers[0].Env = env.MergeEnvs(deployment.Spec.Template.Spec.InitContainers[0].Env, envVars)

	return deployment
}
