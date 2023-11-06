/*
Copyright paskal.maksim@gmail.com
Licensed under the Apache License, Version 2.0 (the "License")
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package e2e_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/maksim-paskal/hcloud-k8s-ctl/internal"
	"github.com/maksim-paskal/hcloud-k8s-ctl/pkg/api"
	"github.com/maksim-paskal/hcloud-k8s-ctl/pkg/config"
	"github.com/maksim-paskal/hcloud-k8s-ctl/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func Test(t *testing.T) { //nolint:funlen,paralleltest,cyclop
	t.Log("Starting e2e tests...")

	log.SetLevel(log.WarnLevel)

	tests, err := filepath.Glob("./configs/*.yaml")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
	defer cancel()

	interruptionSignal := make(chan os.Signal, 1)
	signal.Notify(interruptionSignal, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-interruptionSignal
		log.Warn("Received interruption signal, stopping...")
		cancel()
		<-interruptionSignal
		os.Exit(1)
	}()

	for _, test := range tests { //nolint:paralleltest
		testFile := test
		testName := filepath.Base(testFile)

		if ctx.Err() != nil {
			t.Fatal(ctx.Err())
		}

		t.Run(testName, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
			defer cancel()

			if err := flag.Set("config", testFile); err != nil {
				t.Fatal(err)
			}

			if err := internal.Init(); err != nil {
				t.Fatal(err)
			}

			tmpFile, err := os.CreateTemp("", "kubeconfig")
			if err != nil {
				t.Fatal(err)
			}

			extraDeployments := make(map[interface{}]interface{})
			extraDeployments["nfs"] = map[interface{}]interface{}{
				"server": map[interface{}]interface{}{
					"enabled": true,
				},
			}

			extraDeployments["registry"] = map[interface{}]interface{}{
				"enabled": true,
				"service": map[interface{}]interface{}{
					"type": "LoadBalancer",
				},
			}

			config.SetBranch(os.Getenv("GIT_BRANCH"))
			config.Get().Deployments = extraDeployments
			config.Get().KubeConfigPath = tmpFile.Name()

			applicationAPI, err := api.NewApplicationAPI(ctx)
			if err != nil {
				t.Fatal(err)
			}

			applicationAPI.DeleteCluster(context.Background()) //nolint:contextcheck

			utils.SleepContext(ctx, 10*time.Second)

			t.Log("Creating cluster...")
			if err := applicationAPI.NewCluster(ctx); err != nil {
				t.Fatal(err)
			}

			// test kuberentes cluster
			t.Log("Test kubernetes cluster...")
			if err := testKubernetesCluster(ctx, t, config.Get().KubeConfigPath); err != nil {
				t.Fatal(err)
			}

			// add additional deployments, to test patching
			extraDeployments["nfs"] = map[interface{}]interface{}{
				"server": map[interface{}]interface{}{
					"enabled": true,
				},
				"nfs-subdir-external-provisioner": map[interface{}]interface{}{
					"enabled": true,
				},
			}

			t.Log("Patch cluster...")
			if err := applicationAPI.PatchClusterDeployment(ctx); err != nil {
				t.Fatal(err)
			}

			// wait for patching
			utils.SleepContext(ctx, 10*time.Second)

			// test kuberentes cluster after patching
			t.Log("Test kubernetes cluster...")
			if err := testKubernetesCluster(ctx, t, config.Get().KubeConfigPath); err != nil {
				t.Fatal(err)
			}

			// delete cluster after test
			applicationAPI.DeleteCluster(context.Background()) //nolint:contextcheck
		})
	}
}

type clusterResults struct {
	PodReady,
	PodNotReady,
	PVBound,
	PVNotBound,
	NodeRunning,
	NodeNotRunning,
	LoadBalancerIngress int
}

func (c clusterResults) Validate() bool {
	return c.PodReady > 0 &&
		c.PodNotReady == 0 &&
		c.PVBound > 0 &&
		c.PVNotBound == 0 &&
		c.NodeRunning > 0 &&
		c.NodeNotRunning == 0 &&
		c.LoadBalancerIngress == 3 // hloud allocate 3 ip addresses for load balancer
}

func (c clusterResults) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		return err.Error()
	}

	return string(b)
}

// test kubernetes cluster
// need all pods to be ready
// need all PVs to be bound
// need all nodes to be running
// need all load balancers have 3 ingress ips.
func testKubernetesCluster(ctx context.Context, t *testing.T, kubeconfig string) error { //nolint:funlen,gocognit,cyclop,lll
	t.Helper()

	t.Logf("Using kubeconfig: %s", kubeconfig)

	restconfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return errors.Wrap(err, "error in clientcmd.BuildConfigFromFlags")
	}

	clientset, err := kubernetes.NewForConfig(restconfig)
	if err != nil {
		return errors.Wrap(err, "error in kubernetes.NewForConfig")
	}

	for ctx.Err() == nil {
		result := clusterResults{}

		pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return errors.Wrap(err, "error in clientset.CoreV1().Pods")
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				result.PodNotReady++
			} else {
				result.PodReady++
			}
		}

		pvs, err := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return errors.Wrap(err, "error in clientset.CoreV1().PersistentVolumes")
		}

		for _, pv := range pvs.Items {
			if pv.Status.Phase != corev1.VolumeBound {
				result.PVNotBound++
			} else {
				result.PVBound++
			}
		}

		nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return errors.Wrap(err, "error in clientset.CoreV1().Pods")
		}

		for _, node := range nodes.Items {
			nodeReady := false

			for _, condition := range node.Status.Conditions {
				if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
					nodeReady = true

					break
				}
			}

			if nodeReady {
				result.NodeRunning++
			} else {
				result.NodeNotRunning++
			}
		}

		services, err := clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return errors.Wrap(err, "error in clientset.CoreV1().Services")
		}

		for _, service := range services.Items {
			if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
				result.LoadBalancerIngress += len(service.Status.LoadBalancer.Ingress)
			}
		}

		t.Log(result.String())

		if validate := result.Validate(); validate {
			t.Log("Cluster is OK")

			break
		}

		utils.SleepContext(ctx, 10*time.Second)
	}

	return nil
}
