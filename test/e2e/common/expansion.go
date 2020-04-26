/*
Copyright 2016 The Kubernetes Authors.

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

package common

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	imageutils "k8s.io/kubernetes/test/utils/image"

	"github.com/onsi/ginkgo"
)

// These tests exercise the Kubernetes expansion syntax $(VAR).
// For more information, see:
// https://github.com/kubernetes/community/blob/master/contributors/design-proposals/node/expansion.md
var _ = framework.KubeDescribe("Variable Expansion", func() {
	f := framework.NewDefaultFramework("var-expansion")

	/*
		Release : v1.9
		Testname: Environment variables, expansion
		Description: Create a Pod with environment variables. Environment variables defined using previously defined environment variables MUST expand to proper values.
	*/
	framework.ConformanceIt("should allow composing env vars into new env vars [NodeConformance]", func() {
		envVars := []v1.EnvVar{
			{
				Name:  "FOO",
				Value: "foo-value",
			},
			{
				Name:  "BAR",
				Value: "bar-value",
			},
			{
				Name:  "FOOBAR",
				Value: "$(FOO);;$(BAR)",
			},
		}
		pod := newPod([]string{"sh", "-c", "env"}, envVars, nil, nil)

		f.TestContainerOutput("env composition", pod, 0, []string{
			"FOO=foo-value",
			"BAR=bar-value",
			"FOOBAR=foo-value;;bar-value",
		})
	})

	/*
		Release : v1.9
		Testname: Environment variables, command expansion
		Description: Create a Pod with environment variables and container command using them. Container command using the  defined environment variables MUST expand to proper values.
	*/
	framework.ConformanceIt("should allow substituting values in a container's command [NodeConformance]", func() {
		envVars := []v1.EnvVar{
			{
				Name:  "TEST_VAR",
				Value: "test-value",
			},
		}
		pod := newPod([]string{"sh", "-c", "TEST_VAR=wrong echo \"$(TEST_VAR)\""}, envVars, nil, nil)

		f.TestContainerOutput("substitution in container's command", pod, 0, []string{
			"test-value",
		})
	})

	/*
		Release : v1.9
		Testname: Environment variables, command argument expansion
		Description: Create a Pod with environment variables and container command arguments using them. Container command arguments using the  defined environment variables MUST expand to proper values.
	*/
	framework.ConformanceIt("should allow substituting values in a container's args [NodeConformance]", func() {
		envVars := []v1.EnvVar{
			{
				Name:  "TEST_VAR",
				Value: "test-value",
			},
		}
		pod := newPod([]string{"sh", "-c"}, envVars, nil, nil)
		pod.Spec.Containers[0].Args = []string{"TEST_VAR=wrong echo \"$(TEST_VAR)\""}

		f.TestContainerOutput("substitution in container's args", pod, 0, []string{
			"test-value",
		})
	})

	/*
		Release : v1.19
		Testname: VolumeSubpathEnvExpansion, subpath expansion
		Description: Make sure a container's subpath can be set using an expansion of environment variables.
	*/
	framework.ConformanceIt("should allow substituting values in a volume subpath [sig-storage]", func() {
		envVars := []v1.EnvVar{
			{
				Name:  "POD_NAME",
				Value: "foo",
			},
		}
		mounts := []v1.VolumeMount{
			{
				Name:        "workdir1",
				MountPath:   "/logscontainer",
				SubPathExpr: "$(POD_NAME)",
			},
			{
				Name:      "workdir1",
				MountPath: "/testcontainer",
			},
		}
		volumes := []v1.Volume{
			{
				Name: "workdir1",
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			},
		}
		pod := newPod([]string{}, envVars, mounts, volumes)
		envVars[0].Value = pod.ObjectMeta.Name
		pod.Spec.Containers[0].Command = []string{"sh", "-c", "test -d /testcontainer/" + pod.ObjectMeta.Name + ";echo $?"}

		f.TestContainerOutput("substitution in volume subpath", pod, 0, []string{
			"0",
		})
	})

	/*
		Release : v1.19
		Testname: VolumeSubpathEnvExpansion, subpath with backticks
		Description: Make sure a container's subpath can not be set using an expansion of environment variables when backticks are supplied.
	*/
	framework.ConformanceIt("should fail substituting values in a volume subpath with backticks [sig-storage][Slow]", func() {

		envVars := []v1.EnvVar{
			{
				Name:  "POD_NAME",
				Value: "..",
			},
		}
		mounts := []v1.VolumeMount{
			{
				Name:        "workdir1",
				MountPath:   "/logscontainer",
				SubPathExpr: "$(POD_NAME)",
			},
		}
		volumes := []v1.Volume{
			{
				Name: "workdir1",
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			},
		}
		pod := newPod(nil, envVars, mounts, volumes)

		// Pod should fail
		testPodFailSubpath(f, pod)
	})

	/*
		Release : v1.19
		Testname: VolumeSubpathEnvExpansion, subpath with absolute path
		Description: Make sure a container's subpath can not be set using an expansion of environment variables when absolute path is supplied.
	*/
	framework.ConformanceIt("should fail substituting values in a volume subpath with absolute path [sig-storage][Slow]", func() {

		envVars := []v1.EnvVar{
			{
				Name:  "POD_NAME",
				Value: "/tmp",
			},
		}
		mounts := []v1.VolumeMount{
			{
				Name:        "workdir1",
				MountPath:   "/logscontainer",
				SubPathExpr: "$(POD_NAME)",
			},
		}
		volumes := []v1.Volume{
			{
				Name: "workdir1",
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			},
		}
		pod := newPod(nil, envVars, mounts, volumes)

		// Pod should fail
		testPodFailSubpath(f, pod)
	})

	/*
		Release : v1.19
		Testname: VolumeSubpathEnvExpansion, subpath ready from failed state
		Description: Verify that a failing subpath expansion can be modified during the lifecycle of a container.
	*/
	framework.ConformanceIt("should verify that a failing subpath expansion can be modified during the lifecycle of a container [sig-storage][Slow]", func() {

		envVars := []v1.EnvVar{
			{
				Name:  "POD_NAME",
				Value: "foo",
			},
			{
				Name: "ANNOTATION",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.annotations['mysubpath']",
					},
				},
			},
		}
		mounts := []v1.VolumeMount{
			{
				Name:        "workdir1",
				MountPath:   "/subpath_mount",
				SubPathExpr: "$(ANNOTATION)/$(POD_NAME)",
			},
			{
				Name:      "workdir1",
				MountPath: "/volume_mount",
			},
		}
		volumes := []v1.Volume{
			{
				Name: "workdir1",
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			},
		}
		pod := newPod([]string{"sh", "-c", "tail -f /dev/null"}, envVars, mounts, volumes)
		pod.ObjectMeta.Annotations = map[string]string{"notmysubpath": "mypath"}

		ginkgo.By("creating the pod with failed condition")
		var podClient *framework.PodClient = f.PodClient()
		pod = podClient.Create(pod)

		err := e2epod.WaitTimeoutForPodRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace, framework.PodStartShortTimeout)
		framework.ExpectError(err, "while waiting for pod to be running")

		ginkgo.By("updating the pod")
		podClient.Update(pod.ObjectMeta.Name, func(pod *v1.Pod) {
			pod.ObjectMeta.Annotations = map[string]string{"mysubpath": "mypath"}
		})

		ginkgo.By("waiting for pod running")
		err = e2epod.WaitTimeoutForPodRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace, framework.PodStartShortTimeout)
		framework.ExpectNoError(err, "while waiting for pod to be running")

		ginkgo.By("deleting the pod gracefully")
		err = e2epod.DeletePodWithWait(f.ClientSet, pod)
		framework.ExpectNoError(err, "failed to delete pod")
	})

	/*
		Release : v1.19
		Testname: VolumeSubpathEnvExpansion, subpath test writes
		Description: Verify that a subpath expansion can be used to write files into subpaths.
		1.	valid subpathexpr starts a container running
		2.	test for valid subpath writes
		3.	successful expansion of the subpathexpr isn't required for volume cleanup

	*/
	framework.ConformanceIt("should succeed in writing subpaths in container [sig-storage][Slow]", func() {

		envVars := []v1.EnvVar{
			{
				Name:  "POD_NAME",
				Value: "foo",
			},
			{
				Name: "ANNOTATION",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.annotations['mysubpath']",
					},
				},
			},
		}
		mounts := []v1.VolumeMount{
			{
				Name:        "workdir1",
				MountPath:   "/subpath_mount",
				SubPathExpr: "$(ANNOTATION)/$(POD_NAME)",
			},
			{
				Name:      "workdir1",
				MountPath: "/volume_mount",
			},
		}
		volumes := []v1.Volume{
			{
				Name: "workdir1",
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			},
		}
		pod := newPod([]string{"sh", "-c", "tail -f /dev/null"}, envVars, mounts, volumes)
		pod.ObjectMeta.Annotations = map[string]string{"mysubpath": "mypath"}

		ginkgo.By("creating the pod")
		var podClient *framework.PodClient = f.PodClient()
		pod = podClient.Create(pod)

		ginkgo.By("waiting for pod running")
		err := e2epod.WaitTimeoutForPodRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace, framework.PodStartShortTimeout)
		framework.ExpectNoError(err, "while waiting for pod to be running")

		ginkgo.By("creating a file in subpath")
		cmd := "touch /volume_mount/mypath/foo/test.log"
		_, _, err = f.ExecShellInPodWithFullOutput(pod.Name, cmd)
		if err != nil {
			framework.Failf("expected to be able to write to subpath")
		}

		ginkgo.By("test for file in mounted path")
		cmd = "test -f /subpath_mount/test.log"
		_, _, err = f.ExecShellInPodWithFullOutput(pod.Name, cmd)
		if err != nil {
			framework.Failf("expected to be able to verify file")
		}

		ginkgo.By("updating the annotation value")
		podClient.Update(pod.ObjectMeta.Name, func(pod *v1.Pod) {
			pod.ObjectMeta.Annotations["mysubpath"] = "mynewpath"
		})

		ginkgo.By("waiting for annotated pod running")
		err = e2epod.WaitTimeoutForPodRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace, framework.PodStartShortTimeout)
		framework.ExpectNoError(err, "while waiting for annotated pod to be running")

		ginkgo.By("deleting the pod gracefully")
		err = e2epod.DeletePodWithWait(f.ClientSet, pod)
		framework.ExpectNoError(err, "failed to delete pod")
	})

	/*
		Release : v1.19
		Testname: VolumeSubpathEnvExpansion, subpath lifecycle
		Description: Verify should not change the subpath mount on a container restart if the environment variable changes
		1.	valid subpathexpr starts a container running
		2.	test for valid subpath writes
		3.	container restarts
		4.	delete cleanly
	*/

	framework.ConformanceIt("should not change the subpath mount on a container restart if the environment variable changes [sig-storage][Slow]", func() {
		envVars := []v1.EnvVar{
			{
				Name: "POD_NAME",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.annotations['mysubpath']",
					},
				},
			},
		}
		mounts := []v1.VolumeMount{
			{
				Name:      "workdir1",
				MountPath: "/subpath_mount",
			},
			{
				Name:      "workdir1",
				MountPath: "/volume_mount",
			},
		}
		subpathMounts := []v1.VolumeMount{
			{
				Name:        "workdir1",
				MountPath:   "/subpath_mount",
				SubPathExpr: "$(POD_NAME)",
			},
			{
				Name:      "workdir1",
				MountPath: "/volume_mount",
			},
		}
		volumes := []v1.Volume{
			{
				Name: "workdir1",
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			},
		}
		pod := newPod([]string{"/bin/sh", "-ec", "sleep 100000"}, envVars, subpathMounts, volumes)
		pod.Spec.RestartPolicy = v1.RestartPolicyOnFailure
		pod.ObjectMeta.Annotations = map[string]string{"mysubpath": "foo"}
		suffix := string(uuid.NewUUID())
		pod.Spec.InitContainers = []v1.Container{newContainer(
			fmt.Sprintf("init-volume-%s", suffix), []string{"sh", "-c", "mkdir -p /volume_mount/foo; touch /volume_mount/foo/test.log"}, nil, mounts)}

		// Add liveness probe to subpath container
		pod.Spec.Containers[0].LivenessProbe = &v1.Probe{
			Handler: v1.Handler{
				Exec: &v1.ExecAction{

					Command: []string{"cat", "/subpath_mount/test.log"},
				},
			},
			InitialDelaySeconds: 1,
			FailureThreshold:    1,
			PeriodSeconds:       2,
		}

		// Start pod
		ginkgo.By(fmt.Sprintf("Creating pod %s", pod.Name))
		var podClient *framework.PodClient = f.PodClient()
		pod = podClient.Create(pod)
		defer func() {
			e2epod.DeletePodWithWait(f.ClientSet, pod)
		}()
		err := e2epod.WaitForPodRunningInNamespace(f.ClientSet, pod)
		framework.ExpectNoError(err, "while waiting for pod to be running")

		ginkgo.By("updating the pod")
		podClient.Update(pod.ObjectMeta.Name, func(pod *v1.Pod) {
			pod.ObjectMeta.Annotations = map[string]string{"mysubpath": "newsubpath"}
		})

		ginkgo.By("waiting for pod and container restart")
		waitForPodContainerRestart(f, pod, "/volume_mount/foo/test.log")

		ginkgo.By("test for subpath mounted with old value")
		cmd := "test -f /volume_mount/foo/test.log"
		_, _, err = f.ExecShellInPodWithFullOutput(pod.Name, cmd)
		if err != nil {
			framework.Failf("expected to be able to verify old file exists")
		}

		cmd = "test ! -f /volume_mount/newsubpath/test.log"
		_, _, err = f.ExecShellInPodWithFullOutput(pod.Name, cmd)
		if err != nil {
			framework.Failf("expected to be able to verify new file does not exist")
		}
	})
})

func testPodFailSubpath(f *framework.Framework, pod *v1.Pod) {
	var podClient *framework.PodClient = f.PodClient()
	pod = podClient.Create(pod)

	defer func() {
		e2epod.DeletePodWithWait(f.ClientSet, pod)
	}()

	err := e2epod.WaitTimeoutForPodRunningInNamespace(f.ClientSet, pod.Name, pod.Namespace, framework.PodStartShortTimeout)
	framework.ExpectError(err, "while waiting for pod to be running")
}

// Tests that the existing subpath mount is detected when a container restarts
func waitForPodContainerRestart(f *framework.Framework, pod *v1.Pod, volumeMount string) {

	ginkgo.By("Failing liveness probe")
	stdout, stderr, err := f.ExecShellInPodWithFullOutput(pod.Name, fmt.Sprintf("rm %v", volumeMount))

	framework.Logf("Pod exec output: %v / %v", stdout, stderr)
	framework.ExpectNoError(err, "while failing liveness probe")

	// Check that container has restarted
	ginkgo.By("Waiting for container to restart")
	restarts := int32(0)
	err = wait.PollImmediate(10*time.Second, 2*time.Minute, func() (bool, error) {
		pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == pod.Spec.Containers[0].Name {
				framework.Logf("Container %v, restarts: %v", status.Name, status.RestartCount)
				restarts = status.RestartCount
				if restarts > 0 {
					framework.Logf("Container has restart count: %v", restarts)
					return true, nil
				}
			}
		}
		return false, nil
	})
	framework.ExpectNoError(err, "while waiting for container to restart")

	// Fix liveness probe
	ginkgo.By("Rewriting the file")
	stdout, _, err = f.ExecShellInPodWithFullOutput(pod.Name, fmt.Sprintf("echo test-after > %v", volumeMount))
	framework.Logf("Pod exec output: %v", stdout)
	framework.ExpectNoError(err, "while rewriting the probe file")

	// Wait for container restarts to stabilize
	ginkgo.By("Waiting for container to stop restarting")
	stableCount := int(0)
	stableThreshold := int(time.Minute / framework.Poll)
	err = wait.PollImmediate(framework.Poll, 2*time.Minute, func() (bool, error) {
		pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == pod.Spec.Containers[0].Name {
				if status.RestartCount == restarts {
					stableCount++
					if stableCount > stableThreshold {
						framework.Logf("Container restart has stabilized")
						return true, nil
					}
				} else {
					restarts = status.RestartCount
					stableCount = 0
					framework.Logf("Container has restart count: %v", restarts)
				}
				break
			}
		}
		return false, nil
	})
	framework.ExpectNoError(err, "while waiting for container to stabilize")
}

func newPod(command []string, envVars []v1.EnvVar, mounts []v1.VolumeMount, volumes []v1.Volume) *v1.Pod {
	podName := "var-expansion-" + string(uuid.NewUUID())
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   podName,
			Labels: map[string]string{"name": podName},
		},
		Spec: v1.PodSpec{
			Containers:    []v1.Container{newContainer("dapi-container", command, envVars, mounts)},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes:       volumes,
		},
	}
}

func newContainer(containerName string, command []string, envVars []v1.EnvVar, mounts []v1.VolumeMount) v1.Container {
	return v1.Container{
		Name:         containerName,
		Image:        imageutils.GetE2EImage(imageutils.BusyBox),
		Command:      command,
		Env:          envVars,
		VolumeMounts: mounts,
	}
}
