// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build e2e
// +build e2e

package k8sattributesprocessor_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var (
	testenv   env.Environment
	namespace = "default"
)

func TestMain(m *testing.M) {
	testenv = env.New()
	path := conf.ResolveKubeConfigFile()
	cfg := envconf.NewWithKubeConfig(path).WithNamespace(namespace)

	testenv = env.NewWithConfig(cfg)

	// Use pre-defined environment funcs to create a kind cluster prior to test run
	testenv.Setup()

	// Use pre-defined environment funcs to teardown kind cluster after tests
	testenv.Finish(
	//envfuncs.DeleteNamespace(namespace),
	)

	// launch package tests
	os.Exit(testenv.Run(m))
}

func TestKubernetes(t *testing.T) {
	testdata := os.DirFS("testdata/e2e/")
	pattern := "*.yaml"
	// feature uses pre-generated namespace (see TestMain)
	depFeature := features.New("appsv1/deployment").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r, err := resources.New(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatal(err)
			}
			if err := decoder.DecodeEachFile(ctx, testdata, pattern,
				decoder.CreateIgnoreAlreadyExists(r),
				decoder.MutateNamespace(namespace),
				MutateOtelCollectorImage(os.Getenv("IMAGE")),
			); err != nil {
				t.Fatal(err)
			}

			time.Sleep(2 * time.Second)
			return ctx
		}).
		Assess("deployment creation", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			dep := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "example-opentelemetry-collector", Namespace: cfg.Namespace()},
			}

			err = wait.For(conditions.New(client.Resources()).ResourceMatch(&dep, func(object k8s.Object) bool {
				d := object.(*appsv1.Deployment)
				return d.Status.ReadyReplicas == *d.Spec.Replicas
			}), wait.WithTimeout(time.Minute*2))

			demoApp := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "example-emailservice", Namespace: cfg.Namespace()},
			}

			err = wait.For(conditions.New(client.Resources()).ResourceMatch(&demoApp, func(object k8s.Object) bool {
				d := object.(*appsv1.Deployment)
				return d.Status.ReadyReplicas == *d.Spec.Replicas
			}), wait.WithTimeout(time.Minute*2))

			pods := &corev1.PodList{}
			err = client.Resources(cfg.Namespace()).List(context.TODO(), pods)
			if err != nil || pods.Items == nil {
				t.Error("error while getting pods", err)
			}
			fmt.Println("1")
			fmt.Println(pods)

			return context.WithValue(ctx, "test-deployment", &dep)
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r, err := resources.New(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatal(err)
			}

			if err := decoder.DecodeEachFile(ctx, testdata, pattern,
				decoder.DeleteHandler(r),
				decoder.MutateNamespace(namespace),
			); err != nil {
				t.Fatal(err)
			}

			return ctx
		}).Feature()

	testenv.Test(t, depFeature)
}

func MutateOtelCollectorImage(image string) decoder.DecodeOption {
	return decoder.MutateOption(func(obj k8s.Object) error {
		d, ok := obj.(*appsv1.Deployment)
		if !ok {
			return nil
		}
		if d.Name != "example-opentelemetry-collector" {
			return nil
		}

		fmt.Println("running", image)
		d.Spec.Template.Spec.Containers[0].Image = image
		return nil
	})
}
