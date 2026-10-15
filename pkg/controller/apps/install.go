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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"helm.sh/helm/v3/pkg/release"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/chartstore"
	"github.com/caoyingjunz/pixiu/pkg/controller/cluster"
	"github.com/caoyingjunz/pixiu/pkg/controller/helm"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// Install 异步安装 chart 到指定集群命名空间。
// 同步段：鉴权集群访问 → chart/版本校验 → 查重；随后创建 pending 记录并立即返回，
// 真正的 materialize / 安全预检 / helm 安装由后台 worker 执行。
func (c *controller) Install(ctx context.Context, clusterName, namespace string, req *types.InstallReleaseRequest) (*types.InstallInfo, error) {
	if err := c.ensureStore(); err != nil {
		return nil, apierrors.NewError(err, http.StatusServiceUnavailable)
	}
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}
	// 必须使用鉴权返回的集群行：受限用户传主集群名时，AuthorizeClusterAccessByName 会回落到其
	// 授权子集群（scoped 凭证）。若仍用请求集群名去调 helm，会按主集群名加载出主集群 admin
	// kubeconfig，从而以 cluster-admin 安装任意 chart —— 提权。故后续一律使用 resolved.Name，
	// 并把解析后的集群名落到记录里，供 worker 无用户 ctx 时复用。
	resolved, err := cluster.NewCluster(c.cc, c.factory).AuthorizeClusterAccessByName(ctx, user, clusterName)
	if err != nil {
		return nil, err
	}
	clusterName = resolved.Name

	chart, err := c.factory.Chart().GetChart(ctx, req.ChartId)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	if chart == nil {
		return nil, apierrors.NewError(fmt.Errorf("chart not found"), http.StatusNotFound)
	}
	cv, err := c.resolveVersion(ctx, req.ChartId, req.Version)
	if err != nil {
		return nil, err
	}

	// 同一集群/命名空间下同名 release 不允许重复记账（按解析后的集群名记账，保证与后续操作一致）
	existing, err := c.factory.Chart().GetInstallByTriple(ctx, clusterName, namespace, req.ReleaseName)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	if existing != nil {
		rec := existing
		// 仅「失败」的记录允许复用重试（对齐「重试=复用同一条记录」口径，唯一索引保留不新建）；
		// pending/installing/success 仍视为冲突。dry-run 预览不入队，沿用冲突语义。
		if rec.DeployStatus != model.InstallDeployFailed || req.Preview {
			return nil, apierrors.NewError(
				fmt.Errorf("release %s already exists in %s/%s", req.ReleaseName, clusterName, namespace),
				http.StatusConflict,
			)
		}
		// 复用同一条记录重新入队：置回 pending + install，清空 message/status，
		// 并更新目标 chart/version/values（允许失败后换 chart/版本重试）。
		values, _ := json.Marshal(req.Values)
		if err = c.factory.Chart().UpdateInstallDeploy(ctx, rec.Id, map[string]interface{}{
			"chart_id":      chart.Id,
			"chart_name":    chart.Name,
			"version":       cv.Version,
			"values":        string(values),
			"deploy_status": model.InstallDeployPending,
			"pending_op":    model.InstallOpInstall,
			"message":       "",
			"status":        "",
		}); err != nil {
			return nil, apierrors.ErrServerInternal
		}
		fresh, err := c.factory.Chart().GetInstall(ctx, rec.Id)
		if err != nil || fresh == nil {
			return nil, apierrors.ErrServerInternal
		}
		info := toInstallInfo(fresh)
		return &info, nil
	}

	// dry-run 预览不落地：保持同步渲染（dry-run 本身不产生集群资源、也不记账，故不入队）
	if req.Preview {
		return c.previewInstall(ctx, chart, cv, clusterName, namespace, req, user)
	}

	values, _ := json.Marshal(req.Values)
	record := &model.AppstoreInstall{
		ChartId:       chart.Id,
		ChartName:     chart.Name,
		Version:       cv.Version,
		Cluster:       clusterName,
		Namespace:     namespace,
		ReleaseName:   req.ReleaseName,
		Values:        string(values),
		DeployStatus:  model.InstallDeployPending,
		PendingOp:     model.InstallOpInstall,
		CreatedBy:     user.Id,
		CreatedByName: user.Name,
	}
	if record, err = c.factory.Chart().EnqueueInstall(ctx, record); err != nil {
		return nil, apierrors.ErrServerInternal
	}
	info := toInstallInfo(record)
	return &info, nil
}

// previewInstall 同步执行 helm dry-run（preview）：只渲染不落地，也不产生记账记录。
func (c *controller) previewInstall(ctx context.Context, chart *model.Chart, cv *model.AppstoreChartVersion, clusterName, namespace string, req *types.InstallReleaseRequest, user *model.User) (*types.InstallInfo, error) {
	path, cleanup, err := c.materialize(ctx, cv.StorageKey)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	if err = c.guard(path, req.ReleaseName, namespace, req.Values, isAdmin(user)); err != nil {
		return nil, err
	}

	rel, err := c.helmSvc.Release(clusterName, namespace).Install(ctx, &types.Release{
		Name:    req.ReleaseName,
		Chart:   path,
		Version: cv.Version,
		Values:  req.Values,
		Preview: true, // helm dry-run：只渲染不落地
	})
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("failed to install chart: %w", err), http.StatusInternalServerError)
	}
	info := toInstallInfo(&model.AppstoreInstall{
		ChartId:       chart.Id,
		ChartName:     chart.Name,
		Version:       cv.Version,
		Cluster:       clusterName,
		Namespace:     namespace,
		ReleaseName:   req.ReleaseName,
		Status:        releaseStatus(rel),
		CreatedBy:     user.Id,
		CreatedByName: user.Name,
	})
	return &info, nil
}

// ListReleases 以集群内实时 release 为准列出 release，并叠加平台记账。
// namespace 为空表示「全部命名空间」（内部用 helm 全命名空间哨兵触发跨命名空间列举）。
//
// 数据来源＝集群内实时 release（c.helmSvc.Release(resolvedName, ns).List），逐条按
// (cluster, namespace, release_name) 三元组叠加 DB 记账（Managed=true + 归属/DeployStatus/
// ChartName/Message）；集群内不存在对应记账的外部 release 也会列出，但 Managed=false（只读）。
// 另外，DB 中 deploy_status 为 pending/installing 但集群尚无该 release 的记录也要列出（Managed=true）。
func (c *controller) ListReleases(ctx context.Context, clusterName, namespace string) ([]types.InstallInfo, error) {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}
	// 必须用鉴权返回的集群名（resolved.Name）加载 kubeconfig：受限用户传主集群名时解析到其授权子集群，
	// 若仍用请求集群名，会按主集群名取到 admin 凭证，构成提权。整批列举只查这一次。
	resolved, err := cluster.NewCluster(c.cc, c.factory).AuthorizeClusterAccessByName(ctx, user, clusterName)
	if err != nil {
		return nil, err
	}
	resolvedName := resolved.Name

	// 命名空间为空 → 全部命名空间哨兵；helm 侧据此跨命名空间列举。
	helmNamespace := namespace
	if helmNamespace == "" {
		helmNamespace = helm.AllNamespacesToken
	}
	releases, err := c.helmSvc.Release(resolvedName, helmNamespace).List(ctx)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("failed to list releases: %w", err), http.StatusInternalServerError)
	}

	// DB 记账：按解析后的集群名过滤（指定命名空间时一并下推过滤）。
	opts := []db.Options{db.WithInstallCluster(resolvedName)}
	if namespace != "" {
		opts = append(opts, db.WithNamespace(namespace))
	}
	records, err := c.factory.Chart().ListInstalls(ctx, opts...)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}

	// 记账按 (namespace, release_name) 建索引，供实时叠加与「仅记账」记录补充。
	type triple struct{ namespace, name string }
	recIndex := make(map[triple]*model.AppstoreInstall, len(records))
	for i := range records {
		rec := &records[i]
		recIndex[triple{rec.Namespace, rec.ReleaseName}] = rec
	}

	items := make([]types.InstallInfo, 0, len(releases)+len(records))
	seen := make(map[triple]bool, len(releases))
	for _, rel := range releases {
		if rel == nil || rel.Name == "" {
			continue
		}
		// 跨命名空间列举时仍按请求命名空间收窄（防御性，避免越出请求范围）。
		if namespace != "" && rel.Namespace != namespace {
			continue
		}
		key := triple{rel.Namespace, rel.Name}
		seen[key] = true
		if rec, ok := recIndex[key]; ok {
			info := toInstallInfo(rec) // 记账来源：Managed=true
			info.Status = releaseStatus(rel)
			items = append(items, info)
			continue
		}
		// 外部 release（非平台记账）：只读，Managed=false。
		items = append(items, externalInstallInfo(resolvedName, rel))
	}

	// 补录：DB 中 pending/installing 但集群尚无对应 release 的记账（尚未下发或正在编排）。
	for i := range records {
		rec := &records[i]
		if seen[triple{rec.Namespace, rec.ReleaseName}] {
			continue
		}
		if rec.DeployStatus == model.InstallDeployPending || rec.DeployStatus == model.InstallDeployInstalling {
			items = append(items, toInstallInfo(rec))
		}
	}
	return items, nil
}

// GetRelease 返回单个 release 详情：以集群内实时 release 为准，叠加平台记账（若有）。
func (c *controller) GetRelease(ctx context.Context, clusterName, namespace, name string) (*types.InstallInfo, error) {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}
	resolved, err := cluster.NewCluster(c.cc, c.factory).AuthorizeClusterAccessByName(ctx, user, clusterName)
	if err != nil {
		return nil, err
	}
	resolvedName := resolved.Name

	rel, err := c.helmSvc.Release(resolvedName, namespace).Get(ctx, name)
	if err != nil {
		// helm 侧不存在该 release（含已被卸载）统一按 404 处理。
		return nil, apierrors.NewError(fmt.Errorf("release %s not found in %s/%s", name, resolvedName, namespace), http.StatusNotFound)
	}
	if rel == nil {
		return nil, apierrors.NewError(fmt.Errorf("release %s not found in %s/%s", name, resolvedName, namespace), http.StatusNotFound)
	}

	// 叠加平台记账（若有）：归属/DeployStatus/ChartName/Message 等以记账为准，实时状态以 helm 为准。
	rec, err := c.factory.Chart().GetInstallByTriple(ctx, resolvedName, namespace, name)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	if rec != nil {
		info := toInstallInfo(rec) // Managed=true
		info.Status = releaseStatus(rel)
		return &info, nil
	}
	info := externalInstallInfo(resolvedName, rel)
	return &info, nil
}

// Upgrade 异步升级已安装的 release 到指定版本。
// 同步段：鉴权 → 记账定位（未命中则按需纳管）→ 目标版本解析；随后把目标 version/values 落库并置回 pending 入队。
// 按需纳管：集群内已存在但未记账的外部 release（其 chart 在目录内）会补建 pending 记账并直接入队；
// 若其 chart 不在平台目录（无法解析目标版本），则拒绝（400），避免留下无法执行的 pending 记录。
func (c *controller) Upgrade(ctx context.Context, clusterName, namespace, name string, req *types.UpgradeChartRequest) error {
	if err := c.ensureStore(); err != nil {
		return apierrors.NewError(err, http.StatusServiceUnavailable)
	}
	resolvedName, rec, user, err := c.authorizeAndLoadMaybe(ctx, clusterName, namespace, name)
	if err != nil {
		return err
	}
	if rec == nil {
		// 按需纳管：先构造记账记录（集群内无该 release 会返回 404）。
		rec, err = c.externalReleaseRecord(ctx, resolvedName, namespace, name, model.InstallOpUpgrade, 0, user)
		if err != nil {
			return err
		}
		// 升级依赖目标版本解析（resolveVersion 按 rec.ChartId）；chart 不在目录则无法升级，直接拒绝，不落库。
		if rec.ChartId == 0 {
			return apierrors.NewError(
				fmt.Errorf("该 release 的 chart 不在应用商店目录中，无法通过平台升级；请卸载后用目录 chart 重新安装"),
				http.StatusBadRequest,
			)
		}
		cv, err := c.resolveVersion(ctx, rec.ChartId, req.Version)
		if err != nil {
			return err
		}
		rec.Version = cv.Version // 目标版本，worker 据此 materialize
		if req.Values != nil {
			values, _ := json.Marshal(req.Values)
			rec.Values = string(values)
		}
		if _, err = c.factory.Chart().CreateInstall(ctx, rec); err != nil {
			return apierrors.ErrServerInternal
		}
		return nil
	}

	if err = ensureNotBusy(rec); err != nil {
		return err
	}

	cv, err := c.resolveVersion(ctx, rec.ChartId, req.Version)
	if err != nil {
		return err
	}
	values, _ := json.Marshal(req.Values)
	updates := map[string]interface{}{
		"version":       cv.Version, // 目标版本，worker 据此 materialize
		"values":        string(values),
		"deploy_status": model.InstallDeployPending,
		"pending_op":    model.InstallOpUpgrade,
		"message":       "",
	}
	if err = c.factory.Chart().UpdateInstallDeploy(ctx, rec.Id, updates); err != nil {
		return apierrors.ErrServerInternal
	}
	return nil
}

// Uninstall 异步卸载 release 并删除记账（真正的 helm 卸载 + 删记录由 worker 完成）。
// 同时充当「删除失败记录」入口：worker 侧若集群内已无该 release，会直接清理记账。
// 未记账的外部 release 会先按需纳管（补建 pending=uninstall 记录），再由 worker 执行卸载。
func (c *controller) Uninstall(ctx context.Context, clusterName, namespace, name string) error {
	resolvedName, rec, user, err := c.authorizeAndLoadMaybe(ctx, clusterName, namespace, name)
	if err != nil {
		return err
	}
	if rec == nil {
		rec, err = c.externalReleaseRecord(ctx, resolvedName, namespace, name, model.InstallOpUninstall, 0, user)
		if err != nil {
			return err
		}
		if _, err = c.factory.Chart().CreateInstall(ctx, rec); err != nil {
			return apierrors.ErrServerInternal
		}
		return nil
	}

	if err = ensureNotBusy(rec); err != nil {
		return err
	}
	if err := c.factory.Chart().UpdateInstallDeploy(ctx, rec.Id, map[string]interface{}{
		"deploy_status": model.InstallDeployPending,
		"pending_op":    model.InstallOpUninstall,
		"message":       "",
	}); err != nil {
		return apierrors.ErrServerInternal
	}
	return nil
}

// Rollback 异步回滚 release 到指定 revision。
// 未记账的外部 release 会先按需纳管（补建 pending=rollback 记录，并带上目标 revision）。
func (c *controller) Rollback(ctx context.Context, clusterName, namespace, name string, req *types.RollbackChartRequest) error {
	resolvedName, rec, user, err := c.authorizeAndLoadMaybe(ctx, clusterName, namespace, name)
	if err != nil {
		return err
	}
	if rec == nil {
		rec, err = c.externalReleaseRecord(ctx, resolvedName, namespace, name, model.InstallOpRollback, req.Version, user)
		if err != nil {
			return err
		}
		if _, err = c.factory.Chart().CreateInstall(ctx, rec); err != nil {
			return apierrors.ErrServerInternal
		}
		return nil
	}

	if err = ensureNotBusy(rec); err != nil {
		return err
	}
	if err := c.factory.Chart().UpdateInstallDeploy(ctx, rec.Id, map[string]interface{}{
		"target_revision": req.Version,
		"deploy_status":   model.InstallDeployPending,
		"pending_op":      model.InstallOpRollback,
		"message":         "",
	}); err != nil {
		return apierrors.ErrServerInternal
	}
	return nil
}

// authorizeAndLoadMaybe 鉴权集群访问并尝试读取平台托管的安装记账。
// 返回解析后的集群名（供 helm 加载 kubeconfig，务必使用）、记账记录（可能为 nil）与当前用户。
// 记录未命中不报错，由调用方决定是否按需纳管；身份为集群作用域自然身份 (cluster, namespace, name)。
func (c *controller) authorizeAndLoadMaybe(ctx context.Context, clusterName, namespace, name string) (string, *model.AppstoreInstall, *model.User, error) {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return "", nil, nil, err
	}
	// 必须用鉴权返回的集群名（resolved.Name）加载 kubeconfig：受限用户可能借主集群名解析到其
	// 授权子集群，若仍用请求名会取到主集群 admin 凭证（提权）。防御性一致处理。
	resolved, err := cluster.NewCluster(c.cc, c.factory).AuthorizeClusterAccessByName(ctx, user, clusterName)
	if err != nil {
		return "", nil, nil, err
	}
	rec, err := c.factory.Chart().GetInstallByTriple(ctx, resolved.Name, namespace, name)
	if err != nil {
		return "", nil, nil, apierrors.ErrServerInternal
	}
	return resolved.Name, rec, user, nil
}

// externalReleaseRecord 依据集群内已存在的 release 构造一条「纳管」记账记录（未落库）。
// 字段取自 helm release：ChartName/Version 取 chart 元数据，Values 取 release.Config，
// ChartId 按 chart 名在平台目录 best-effort 匹配（命中回填，否则为 0）。
// 集群内不存在该 release → 404（外部 release 必须先真实存在才能纳管）。
// op 为本次操作；rollback 需传 targetRevision。
func (c *controller) externalReleaseRecord(ctx context.Context, resolvedName, namespace, name, op string, targetRevision int, user *model.User) (*model.AppstoreInstall, error) {
	rel, err := c.helmSvc.Release(resolvedName, namespace).Get(ctx, name)
	if err != nil || rel == nil {
		return nil, apierrors.NewError(
			fmt.Errorf("release %s not found in %s/%s", name, resolvedName, namespace),
			http.StatusNotFound,
		)
	}

	rec := &model.AppstoreInstall{
		Cluster:        resolvedName,
		Namespace:      namespace,
		ReleaseName:    name,
		Status:         releaseStatus(rel),
		DeployStatus:   model.InstallDeployPending,
		PendingOp:      op,
		TargetRevision: targetRevision,
		CreatedBy:      user.Id,
		CreatedByName:  user.Name,
	}
	if rel.Chart != nil && rel.Chart.Metadata != nil {
		rec.ChartName = rel.Chart.Metadata.Name
		rec.Version = rel.Chart.Metadata.Version
	}
	if rel.Config != nil {
		if values, err := json.Marshal(rel.Config); err == nil {
			rec.Values = string(values)
		}
	}
	// best-effort：按 chart 名在平台目录匹配，命中回填 ChartId（供升级解析目标版本）。
	if rec.ChartName != "" {
		if chart, err := c.factory.Chart().GetChartByName(ctx, rec.ChartName); err == nil && chart != nil {
			rec.ChartId = chart.Id
		}
	}
	return rec, nil
}

// ensureNotBusy 拒绝正处于在途编排（installing）的 release，避免并发操作互相覆盖。
func ensureNotBusy(rec *model.AppstoreInstall) error {
	if rec.DeployStatus == model.InstallDeployInstalling {
		return apierrors.NewError(
			fmt.Errorf("release %s is busy with an in-flight operation, please retry later", rec.ReleaseName),
			http.StatusConflict,
		)
	}
	return nil
}

// isAdmin 判定用户是否为管理员（root/admin）。用于安全预检豁免的等价判定。
func isAdmin(user *model.User) bool {
	return user != nil && (user.Role == model.RoleRoot || user.Role == model.RoleAdmin)
}

// History 返回 release 的历史版本列表（只读，直接查集群 helm，不依赖平台记账）。
func (c *controller) History(ctx context.Context, clusterName, namespace, name string) ([]types.InstallHistoryItem, error) {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}
	// 必须用鉴权返回的集群名（resolved.Name）加载 kubeconfig，否则受限用户可借主集群名拿到 admin 凭证（提权）
	resolved, err := cluster.NewCluster(c.cc, c.factory).AuthorizeClusterAccessByName(ctx, user, clusterName)
	if err != nil {
		return nil, err
	}

	history, err := c.helmSvc.Release(resolved.Name, namespace).History(ctx, name)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("failed to get release history: %w", err), http.StatusInternalServerError)
	}

	items := make([]types.InstallHistoryItem, 0, len(history))
	for _, rel := range history {
		item := types.InstallHistoryItem{
			Revision:    rel.Version,
			Version:     rel.Version,
			Status:      string(rel.Info.Status),
			Description: rel.Info.Description,
			Updated:     rel.Info.LastDeployed.Format("2006-01-02 15:04:05"),
		}
		if rel.Chart != nil && rel.Chart.Metadata != nil {
			item.Chart = fmt.Sprintf("%s-%s", rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)
			item.AppVersion = rel.Chart.Metadata.AppVersion
		}
		items = append(items, item)
	}
	return items, nil
}

// materialize 将 chart 包解析为本地文件路径供 helm 使用。
func (c *controller) materialize(ctx context.Context, key string) (string, func(), error) {
	if p, ok := chartstore.AsLocalPath(c.store, key); ok {
		if _, err := os.Stat(p); err != nil {
			return "", func() {}, apierrors.NewError(fmt.Errorf("chart package not found: %w", err), http.StatusNotFound)
		}
		return p, func() {}, nil
	}

	rc, err := c.store.Open(ctx, key)
	if err != nil {
		return "", func() {}, apierrors.NewError(fmt.Errorf("chart package not found: %w", err), http.StatusNotFound)
	}
	defer rc.Close()

	f, err := os.CreateTemp("", "appstore-*.tgz")
	if err != nil {
		return "", func() {}, apierrors.ErrServerInternal
	}
	path := f.Name()
	if _, err = io.Copy(f, rc); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", func() {}, apierrors.ErrServerInternal
	}
	if err = f.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() {}, apierrors.ErrServerInternal
	}
	return path, func() { _ = os.Remove(path) }, nil
}

func releaseStatus(rel *release.Release) string {
	if rel == nil || rel.Info == nil {
		return ""
	}
	return string(rel.Info.Status)
}

// externalInstallInfo 由集群内实时 release 构造「外部 release」信息（非平台记账，只读，Managed=false）。
func externalInstallInfo(clusterName string, rel *release.Release) types.InstallInfo {
	info := types.InstallInfo{
		Cluster:     clusterName,
		Namespace:   rel.Namespace,
		ReleaseName: rel.Name,
		Status:      releaseStatus(rel),
		Managed:     false,
	}
	if rel.Chart != nil && rel.Chart.Metadata != nil {
		info.ChartName = rel.Chart.Metadata.Name
		info.Version = rel.Chart.Metadata.Version
	}
	return info
}

func toInstallInfo(rec *model.AppstoreInstall) types.InstallInfo {
	spec := types.FormatTime(rec.GmtCreate, rec.GmtModified)
	return types.InstallInfo{
		Id:             rec.Id,
		ChartId:        rec.ChartId,
		ChartName:      rec.ChartName,
		Version:        rec.Version,
		Cluster:        rec.Cluster,
		Namespace:      rec.Namespace,
		ReleaseName:    rec.ReleaseName,
		Values:         rec.Values,
		Status:         rec.Status,
		DeployStatus:   rec.DeployStatus,
		PendingOp:      rec.PendingOp,
		Message:        rec.Message,
		Managed:        true, // 记账来源：平台托管
		TargetRevision: rec.TargetRevision,
		CreatedBy:      rec.CreatedBy,
		CreatedByName:  rec.CreatedByName,
		GmtCreate:      spec.GmtCreate.(string),
		GmtModified:    spec.GmtModified.(string),
	}
}
