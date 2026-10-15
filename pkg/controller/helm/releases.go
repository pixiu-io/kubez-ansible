/*
Copyright 2024 The Pixiu Authors.

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

package helm

import (
	"context"
	"fmt"
	"io"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

type ReleaseInterface interface {
	Install(ctx context.Context, form *types.Release) (*release.Release, error)

	Get(ctx context.Context, name string) (*release.Release, error)
	List(ctx context.Context) ([]*release.Release, error)
	Uninstall(ctx context.Context, name string) (*release.UninstallReleaseResponse, error)
	Upgrade(ctx context.Context, form *types.Release) (*release.Release, error)
	History(ctx context.Context, name string) ([]*release.Release, error)
	Rollback(ctx context.Context, name string, toVersion int) error
}

type Releases struct {
	settings     *cli.EnvSettings
	actionConfig *action.Configuration
}

func NewReleases(actionConfig *action.Configuration, settings *cli.EnvSettings) *Releases {
	return &Releases{
		actionConfig: actionConfig,
		settings:     settings,
	}
}

var _ ReleaseInterface = &Releases{}

func (r *Releases) Get(ctx context.Context, name string) (*release.Release, error) {
	client := action.NewGet(r.actionConfig)
	return client.Run(name)
}

func (r *Releases) List(ctx context.Context) ([]*release.Release, error) {
	client := action.NewList(r.actionConfig)
	// settings 命名空间为哨兵值时表示「全部命名空间」。
	// 注意：helm 3.14.2 的 action.List.Run() 并不读取 AllNamespaces 字段
	// （仅在 helm CLI 中作为触发重新 Init 的开关）。真正让列表跨命名空间生效的是
	// Release() 中按空命名空间 Init 的 storage driver。此处设置 AllNamespaces
	// 以对齐 helm CLI 语义并保持前向兼容。
	if r.settings.Namespace() == AllNamespacesToken {
		client.AllNamespaces = true
	}
	return client.Run()
}

// InstallRelease install release
func (r *Releases) Install(ctx context.Context, form *types.Release) (*release.Release, error) {
	client := action.NewInstall(r.actionConfig)
	client.ReleaseName = form.Name
	client.Namespace = r.settings.Namespace()
	client.Version = form.Version

	client.DryRun = form.Preview
	if client.DryRun {
		client.Description = "server"
	}
	chart, err := r.locateChart(client.ChartPathOptions, form.Chart, r.settings)
	if err != nil {
		return nil, err
	}
	out, err := client.Run(chart, form.Values)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Releases) Uninstall(ctx context.Context, name string) (*release.UninstallReleaseResponse, error) {
	client := action.NewUninstall(r.actionConfig)
	return client.Run(name)
}

// UpgradeRelease upgrade release
func (r *Releases) Upgrade(ctx context.Context, form *types.Release) (*release.Release, error) {
	client := action.NewUpgrade(r.actionConfig)
	client.Namespace = r.settings.Namespace()
	client.DryRun = form.Preview
	if client.DryRun {
		client.Description = "server"
	}

	chart, err := r.locateChart(client.ChartPathOptions, form.Chart, r.settings)
	if err != nil {
		return nil, err
	}

	out, err := client.Run(form.Name, chart, form.Values)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Releases) History(ctx context.Context, name string) ([]*release.Release, error) {
	client := action.NewHistory(r.actionConfig)
	return client.Run(name)
}

func (r *Releases) Rollback(ctx context.Context, name string, toVersion int) error {
	klog.Infof("rolling back release %s to version %d", name, toVersion)
	_, err := r.Get(ctx, name)
	if err != nil {
		return err
	}

	client := action.NewRollback(r.actionConfig)
	client.Version = toVersion
	return client.Run(name)
}

func (r *Releases) locateChart(pathOpts action.ChartPathOptions, chart string, settings *cli.EnvSettings) (*chart.Chart, error) {
	// from cmd/helm/install.go and cmd/helm/upgrade.go
	cp, err := pathOpts.LocateChart(chart, settings)
	if err != nil {
		return nil, err
	}

	p := getter.All(settings)

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return nil, err
	}

	if err := checkIfInstallable(chartRequested); err != nil {
		return nil, err
	}

	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(false),
		//registry.ClientOptWriter(out),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to crete helm config object %v", err)
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			err = fmt.Errorf("an error occurred while checking for chart dependencies. You may need to run `helm dependency build` to fetch missing dependencies: %v", err)
			if true { // client.DependencyUpdate
				man := &downloader.Manager{
					Out:              io.Discard,
					ChartPath:        cp,
					Keyring:          pathOpts.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
					Debug:            settings.Debug,
					RegistryClient:   registryClient, // added on top of Helm code
				}
				if err := man.Update(); err != nil {
					return nil, err
				}
				// Reload the chart with the updated Chart.lock file.
				if chartRequested, err = loader.Load(cp); err != nil {
					return nil, fmt.Errorf("failed reloading chart after repo update : %v", err)
				}
			} else {
				return nil, err
			}
		}
	}

	return chartRequested, nil
}

func checkIfInstallable(ch *chart.Chart) error {
	switch ch.Metadata.Type {
	case "", "application":
		return nil
	}
	return fmt.Errorf("%s charts are not installable", ch.Metadata.Type)
}
