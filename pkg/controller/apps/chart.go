/*
Copyright 2026 The Pixiu Authors.

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

package apps

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	helmchart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/chartutil"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

const (
	defaultPageSize = 10
	maxPageSize     = 100
)

// Upload 上传本地 .tgz chart 包（root/admin）。
func (c *controller) Upload(ctx context.Context, _ string, category string, r io.Reader) (*types.ChartInfo, error) {
	if err := c.ensureStore(); err != nil {
		return nil, apierrors.NewError(err, http.StatusServiceUnavailable)
	}
	if err := controllerutil.CheckAdmin(ctx); err != nil {
		return nil, err
	}

	data, err := c.readLimited(r)
	if err != nil {
		return nil, err
	}

	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}
	md, err := chartutil.Parse(data)
	if err != nil {
		return nil, apierrors.NewError(err, http.StatusBadRequest)
	}
	if !chartutil.IsApplication(md) {
		return nil, apierrors.NewError(fmt.Errorf("chart type %q is not installable, only application charts are accepted", md.Type), http.StatusBadRequest)
	}
	if md.Name == "" || md.Version == "" {
		return nil, apierrors.NewError(fmt.Errorf("chart name and version are required"), http.StatusBadRequest)
	}

	return c.persistChart(ctx, md, data, category, "upload", "", user)
}

// ImportFromURL 从 Helm 仓库导入指定 chart 版本（root/admin）。
func (c *controller) ImportFromURL(ctx context.Context, req *types.ImportChartRequest) (*types.ChartInfo, error) {
	if err := c.ensureStore(); err != nil {
		return nil, apierrors.NewError(err, http.StatusServiceUnavailable)
	}
	if err := controllerutil.CheckAdmin(ctx); err != nil {
		return nil, err
	}

	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	data, err := c.fetchRemoteChart(ctx, req)
	if err != nil {
		return nil, apierrors.NewError(err, http.StatusBadRequest)
	}

	md, err := chartutil.Parse(data)
	if err != nil {
		return nil, apierrors.NewError(err, http.StatusBadRequest)
	}
	if !chartutil.IsApplication(md) {
		return nil, apierrors.NewError(fmt.Errorf("chart type %q is not installable, only application charts are accepted", md.Type), http.StatusBadRequest)
	}

	return c.persistChart(ctx, md, data, req.Category, "url", req.RepoUrl, user)
}

// persistChart 上传与导入共用的落库管线：校验重复 → 存储 → 写 charts/chart_versions。
func (c *controller) persistChart(ctx context.Context, md *chartutil.Metadata, data []byte, category, source, repoURL string, user *model.User) (*types.ChartInfo, error) {
	chart, err := c.factory.Chart().GetChartByName(ctx, md.Name)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}

	// 新建目录项
	if chart == nil {
		chart = &model.Chart{
			Name:          md.Name,
			Category:      category,
			Description:   md.Description,
			Icon:          md.Icon,
			Source:        source,
			RepoURL:       repoURL,
			LatestVersion: md.Version,
			CreatedBy:     user.Id,
			CreatedByName: user.Name,
		}
		if chart, err = c.factory.Chart().CreateChart(ctx, chart); err != nil {
			return nil, apierrors.ErrServerInternal
		}
	} else {
		versions, err := c.factory.Chart().ListChartVersions(ctx, chart.Id)
		if err != nil {
			return nil, apierrors.ErrServerInternal
		}
		for _, v := range versions {
			if v.Version == md.Version {
				return nil, apierrors.NewError(fmt.Errorf("chart %s version %s already exists", md.Name, md.Version), http.StatusConflict)
			}
			// sha256 去重：相同内容包（跨版本）拒绝重复入库
			if v.Digest != "" && v.Digest == md.Sha256 {
				return nil, apierrors.NewError(fmt.Errorf("identical chart package already exists as version %s", v.Version), http.StatusConflict)
			}
		}
	}

	key := storeKey(md.Name, md.Version)
	if err = c.store.Save(ctx, key, bytes.NewReader(data)); err != nil {
		return nil, apierrors.NewError(fmt.Errorf("failed to store chart: %w", err), http.StatusInternalServerError)
	}

	version := &model.AppstoreChartVersion{
		ChartId:     chart.Id,
		Version:     md.Version,
		AppVersion:  md.AppVersion,
		Description: md.Description,
		Icon:        md.Icon,
		Digest:      md.Sha256,
		Size:        md.Size,
		StorageKey:  key,
	}
	if _, err = c.factory.Chart().CreateChartVersion(ctx, version); err != nil {
		// 回滚已存储的对象，避免产生孤儿文件
		_ = c.store.Delete(ctx, key)
		return nil, apierrors.ErrServerInternal
	}

	// 更新目录项的展示信息与最新版本
	updates := map[string]interface{}{"latest_version": md.Version}
	if category != "" {
		updates["category"] = category
	}
	if md.Description != "" {
		updates["description"] = md.Description
	}
	if md.Icon != "" {
		updates["icon"] = md.Icon
	}
	if source != "" {
		updates["source"] = source
	}
	if repoURL != "" {
		updates["repo_url"] = repoURL
	}
	if err = c.factory.Chart().UpdateChart(ctx, chart.Id, chart.ResourceVersion, updates); err != nil {
		return nil, apierrors.ErrServerInternal
	}

	return c.Get(ctx, chart.Id)
}

// List 分页查询 chart 目录。
func (c *controller) List(ctx context.Context, opt types.ListChartOption) (*types.ChartListResponse, error) {
	page, pageSize := normalizePaging(opt.Page, opt.PageSize)

	opts := []db.Options{
		db.WithNameLike(opt.Name),
		db.WithChartCategory(opt.Category),
	}
	total, err := c.factory.Chart().CountCharts(ctx, opts...)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}

	opts = append(opts, db.WithOrderByDesc(), db.WithOffset((page-1)*pageSize), db.WithLimit(pageSize))
	charts, err := c.factory.Chart().ListCharts(ctx, opts...)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.ChartInfo, 0, len(charts))
	for i := range charts {
		items = append(items, toChartInfo(&charts[i], nil))
	}

	// 批量判定 installed：一次查询本页所有 chart 的安装记录，避免逐条 N+1
	if len(items) > 0 {
		ids := make([]int64, 0, len(items))
		for i := range items {
			ids = append(ids, items[i].Id)
		}
		installedIds, err := c.factory.Chart().ListInstalledChartIds(ctx, ids)
		if err != nil {
			return nil, apierrors.ErrServerInternal
		}
		installedSet := make(map[int64]struct{}, len(installedIds))
		for _, id := range installedIds {
			installedSet[id] = struct{}{}
		}
		for i := range items {
			if _, ok := installedSet[items[i].Id]; ok {
				items[i].Installed = true
			}
		}
	}

	return &types.ChartListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	}, nil
}

// Get 查看 chart 详情（含全部版本）。
func (c *controller) Get(ctx context.Context, id int64) (*types.ChartInfo, error) {
	chart, versions, err := c.chartFor(ctx, id)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil, apierrors.NewError(fmt.Errorf("chart not found"), http.StatusNotFound)
		}
		return nil, apierrors.ErrServerInternal
	}
	if chart == nil {
		return nil, apierrors.NewError(fmt.Errorf("chart not found"), http.StatusNotFound)
	}
	info := toChartInfo(chart, versions)
	return &info, nil
}

// Delete 删除 chart 目录项（root/admin）。
// 级联策略：存在安装记录时拒绝删除（409），需先在集群卸载；否则删除版本记录 + 目录项 + 存储对象。
func (c *controller) Delete(ctx context.Context, id int64) error {
	if err := controllerutil.CheckAdmin(ctx); err != nil {
		return err
	}

	chart, versions, err := c.chartFor(ctx, id)
	if err != nil {
		return apierrors.ErrServerInternal
	}
	if chart == nil {
		return apierrors.NewError(fmt.Errorf("chart not found"), http.StatusNotFound)
	}

	installs, err := c.factory.Chart().ListInstalls(ctx, db.WithChartId(chart.Id))
	if err != nil {
		return apierrors.ErrServerInternal
	}
	if len(installs) > 0 {
		return apierrors.NewError(
			fmt.Errorf("chart %s is referenced by %d install record(s), uninstall them first", chart.Name, len(installs)),
			http.StatusConflict,
		)
	}

	if err = c.factory.Chart().DeleteChartVersionsByChart(ctx, chart.Id); err != nil {
		return apierrors.ErrServerInternal
	}
	if err = c.factory.Chart().DeleteChart(ctx, chart.Id); err != nil {
		return apierrors.ErrServerInternal
	}

	// 存储对象删除失败只记录（DB 已清理，避免残留目录项）
	if c.store != nil {
		for _, v := range versions {
			_ = c.store.Delete(ctx, v.StorageKey)
		}
	}
	return nil
}

// DefaultValues 返回指定版本的默认 values.yaml 内容。
func (c *controller) DefaultValues(ctx context.Context, id int64, version string) (string, error) {
	if err := c.ensureStore(); err != nil {
		return "", apierrors.NewError(err, http.StatusServiceUnavailable)
	}

	cv, err := c.resolveVersion(ctx, id, version)
	if err != nil {
		return "", err
	}

	rc, err := c.store.Open(ctx, cv.StorageKey)
	if err != nil {
		return "", apierrors.NewError(fmt.Errorf("chart package not found: %w", err), http.StatusNotFound)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", apierrors.ErrServerInternal
	}
	ch, err := loader.LoadArchive(bytes.NewReader(data))
	if err != nil {
		return "", apierrors.NewError(fmt.Errorf("invalid chart archive: %w", err), http.StatusBadRequest)
	}

	// 等价于 `helm show values`：直接返回 chart 内置 values.yaml 的解析结果
	out, err := yaml.Marshal(ch.Values)
	if err != nil {
		return "", apierrors.ErrServerInternal
	}
	return string(out), nil
}

// Templates 返回 chart（最新版本）内的模板文件清单（原始、未渲染）。
func (c *controller) Templates(ctx context.Context, id int64) ([]types.ChartTemplateInfo, error) {
	ch, err := c.loadChart(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]types.ChartTemplateInfo, 0, len(ch.Templates))
	for _, f := range ch.Templates {
		out = append(out, types.ChartTemplateInfo{
			Name: f.Name,
			Kind: templateKind(f.Data),
			Size: int64(len(f.Data)),
		})
	}
	return out, nil
}

// TemplateContent 返回 chart（最新版本）内指定模板文件的原始内容。
func (c *controller) TemplateContent(ctx context.Context, id int64, name string) (string, error) {
	ch, err := c.loadChart(ctx, id)
	if err != nil {
		return "", err
	}
	for _, f := range ch.Templates {
		if f.Name == name {
			return string(f.Data), nil
		}
	}
	return "", apierrors.NewError(fmt.Errorf("template %s not found", name), http.StatusNotFound)
}

// loadChart 取 chart 最新版本的存储包并解析为 helm chart 对象（默认 values / 模板读取共用）。
func (c *controller) loadChart(ctx context.Context, id int64) (*helmchart.Chart, error) {
	if err := c.ensureStore(); err != nil {
		return nil, apierrors.NewError(err, http.StatusServiceUnavailable)
	}
	cv, err := c.resolveVersion(ctx, id, "")
	if err != nil {
		return nil, err
	}
	rc, err := c.store.Open(ctx, cv.StorageKey)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("chart package not found: %w", err), http.StatusNotFound)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	ch, err := loader.LoadArchive(bytes.NewReader(data))
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("invalid chart archive: %w", err), http.StatusBadRequest)
	}
	return ch, nil
}

// templateKind 从模板原文尽力解析 kind（可能含 go template 表达式，取首个 kind: 行的字面值）。
func templateKind(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "kind:") {
			return strings.TrimSpace(strings.TrimPrefix(t, "kind:"))
		}
	}
	return ""
}

// resolveVersion 定位 chart 的指定版本；version 为空时取最新（最新版本记录）。
func (c *controller) resolveVersion(ctx context.Context, id int64, version string) (*model.AppstoreChartVersion, error) {
	chart, err := c.factory.Chart().GetChart(ctx, id)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	if chart == nil {
		return nil, apierrors.NewError(fmt.Errorf("chart not found"), http.StatusNotFound)
	}

	if version == "" {
		version = chart.LatestVersion
	}
	if version == "" {
		return nil, apierrors.NewError(fmt.Errorf("chart %s has no available version", chart.Name), http.StatusNotFound)
	}

	cv, err := c.factory.Chart().GetChartVersion(ctx, chart.Id, version)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	if cv == nil {
		return nil, apierrors.NewError(fmt.Errorf("chart %s version %s not found", chart.Name, version), http.StatusNotFound)
	}
	return cv, nil
}

// readLimited 按 MaxUploadMB 限制读取，超限返回 400。
func (c *controller) readLimited(r io.Reader) ([]byte, error) {
	limit := int64(c.cc.Appstore.MaxUploadMB) << 20
	if limit <= 0 {
		limit = 32 << 20
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("failed to read upload: %w", err), http.StatusBadRequest)
	}
	if int64(len(data)) > limit {
		return nil, apierrors.NewError(fmt.Errorf("chart package exceeds max size %d MB", c.cc.Appstore.MaxUploadMB), http.StatusRequestEntityTooLarge)
	}
	return data, nil
}

// fetchRemoteChart 从远端仓库拉取指定 chart 版本内容（复用 helm 的 repo/getter）。
func (c *controller) fetchRemoteChart(ctx context.Context, req *types.ImportChartRequest) ([]byte, error) {
	entry := &repo.Entry{
		URL:      req.RepoUrl,
		Username: req.Username,
		Password: req.Password,
	}
	client, err := repo.NewChartRepository(entry, getter.All(cli.New()))
	if err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}
	if _, err = client.DownloadIndexFile(); err != nil {
		return nil, fmt.Errorf("failed to download repository index: %w", err)
	}
	if client.IndexFile == nil {
		return nil, fmt.Errorf("repository index is empty")
	}

	entries, ok := client.IndexFile.Entries[req.Chart]
	if !ok || len(entries) == 0 {
		return nil, fmt.Errorf("chart %s not found in repository", req.Chart)
	}
	var target *repo.ChartVersion
	for _, e := range entries {
		if e.Version == req.Version {
			target = e
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("chart %s version %s not found in repository", req.Chart, req.Version)
	}
	if len(target.URLs) == 0 {
		return nil, fmt.Errorf("chart %s version %s has no download url", req.Chart, req.Version)
	}

	tgzURL, err := resolveReferenceURL(req.RepoUrl, target.URLs[0])
	if err != nil {
		return nil, err
	}

	resp, err := client.Client.Get(tgzURL,
		getter.WithURL(req.RepoUrl),
		getter.WithBasicAuth(req.Username, req.Password),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to download chart: %w", err)
	}
	data := resp.Bytes()
	limit := int64(c.cc.Appstore.MaxUploadMB) << 20
	if limit > 0 && int64(len(data)) > limit {
		return nil, fmt.Errorf("chart package exceeds max size %d MB", c.cc.Appstore.MaxUploadMB)
	}
	_ = ctx
	return data, nil
}

// resolveReferenceURL 将 index 中的相对 URL 解析为绝对 URL（对齐 helm 行为）。
func resolveReferenceURL(baseURL, refURL string) (string, error) {
	parsedRef, err := url.Parse(refURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s as URL: %w", refURL, err)
	}
	if parsedRef.IsAbs() {
		return refURL, nil
	}
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s as URL: %w", baseURL, err)
	}
	parsedBase.RawPath = strings.TrimSuffix(parsedBase.RawPath, "/") + "/"
	parsedBase.Path = strings.TrimSuffix(parsedBase.Path, "/") + "/"
	resolved := parsedBase.ResolveReference(parsedRef)
	resolved.RawQuery = parsedBase.RawQuery
	return resolved.String(), nil
}

func normalizePaging(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func toChartInfo(chart *model.Chart, versions []model.AppstoreChartVersion) types.ChartInfo {
	spec := types.FormatTime(chart.GmtCreate, chart.GmtModified)
	info := types.ChartInfo{
		Id:            chart.Id,
		Name:          chart.Name,
		Category:      chart.Category,
		Description:   chart.Description,
		Icon:          chart.Icon,
		Source:        chart.Source,
		RepoUrl:       chart.RepoURL,
		LatestVersion: chart.LatestVersion,
		Downloads:     chart.Downloads,
		InstallCount:  chart.InstallCount,
		CreatedBy:     chart.CreatedBy,
		CreatedByName: chart.CreatedByName,
		GmtCreate:     spec.GmtCreate.(string),
		GmtModified:   spec.GmtModified.(string),
	}
	if versions != nil {
		info.Versions = make([]types.ChartVersionInfo, 0, len(versions))
		for _, v := range versions {
			vs := types.FormatTime(v.GmtCreate, v.GmtModified)
			info.Versions = append(info.Versions, types.ChartVersionInfo{
				Version:     v.Version,
				AppVersion:  v.AppVersion,
				Description: v.Description,
				Icon:        v.Icon,
				Digest:      v.Digest,
				Size:        v.Size,
				GmtCreate:   vs.GmtCreate.(string),
			})
		}
	}
	return info
}
