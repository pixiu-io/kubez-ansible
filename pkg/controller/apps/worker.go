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
	"path/filepath"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// 每轮从队列捞取的待执行记录上限；pollInterval 为无任务时的轮询间隔。
const (
	pendingBatchSize = 20
	pollInterval     = 2 * time.Second
	logPollInterval  = 500 * time.Millisecond
)

// Run 启动应用商店异步编排 worker 池。
// 启动阶段先做状态恢复（把上次进程异常中断遗留的 installing 记录纠正为 failed），
// 再起 workers 个 goroutine 轮询待执行记录。ctx 取消时 goroutine 退出。
func (c *controller) Run(ctx context.Context, workers int) error {
	if workers <= 0 {
		workers = 1
	}

	if n, err := c.factory.Chart().MarkStaleInstallingFailed(ctx); err != nil {
		klog.Errorf("failed to mark stale appstore installs as failed: %v", err)
	} else if n > 0 {
		klog.Infof("marked %d stale appstore install record(s) as failed on startup", n)
	}

	klog.Infof("starting appstore install manager with %d worker(s)", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.worker, pollInterval)
	}
	return nil
}

// worker 单轮：捞取一批 pending 记录并逐个尝试抢占执行。轮询间隔由 wait.UntilWithContext 控制。
func (c *controller) worker(ctx context.Context) {
	records, err := c.factory.Chart().ListPendingInstalls(ctx, pendingBatchSize)
	if err != nil {
		klog.Errorf("failed to list pending appstore installs: %v", err)
		return
	}
	for i := range records {
		c.process(ctx, &records[i])
	}
}

// process 抢占一条记录，抢到才执行（防多 worker/多副本重复执行）。
func (c *controller) process(ctx context.Context, rec *model.AppstoreInstall) {
	claimed, err := c.factory.Chart().ClaimPendingInstall(ctx, rec.Id)
	if err != nil {
		klog.Errorf("failed to claim appstore install(%d): %v", rec.Id, err)
		return
	}
	if !claimed {
		return
	}
	c.execute(ctx, rec)
}

// execute 按 pending_op 分派执行，并把结果回写记录。
// 日志写入 per-install 文件（每次执行开始时截断重写）。
func (c *controller) execute(ctx context.Context, rec *model.AppstoreInstall) {
	sink := c.newLogSink(rec.Id)
	defer sink.close()

	// 本次操作类型：成功回写时会把 pending_op 置空，故先记录下来供后续判定。
	op := rec.PendingOp

	sink.Printf("开始执行操作: %s (cluster=%s namespace=%s release=%s)",
		opLabel(rec.PendingOp), rec.Cluster, rec.Namespace, rec.ReleaseName)

	var (
		helmStatus string
		execErr    error
	)

	switch rec.PendingOp {
	case model.InstallOpInstall:
		helmStatus, execErr = c.doInstall(ctx, rec, sink)
	case model.InstallOpUpgrade:
		helmStatus, execErr = c.doUpgrade(ctx, rec, sink)
	case model.InstallOpRollback:
		helmStatus, execErr = c.doRollback(ctx, rec, sink)
	case model.InstallOpUninstall:
		execErr = c.doUninstall(ctx, rec, sink)
		if execErr == nil {
			// 卸载成功：记录已被删除，无需回写
			sink.Printf("操作完成")
			return
		}
	default:
		execErr = fmt.Errorf("unknown operation type %q", rec.PendingOp)
	}

	if execErr != nil {
		sink.Printf("错误: %v", execErr)
		if err := c.factory.Chart().UpdateInstallDeploy(ctx, rec.Id, map[string]interface{}{
			"deploy_status": model.InstallDeployFailed,
			"message":       execErr.Error(),
		}); err != nil {
			klog.Errorf("failed to mark appstore install(%d) failed: %v", rec.Id, err)
		}
		return
	}

	if err := c.factory.Chart().UpdateInstallDeploy(ctx, rec.Id, map[string]interface{}{
		"deploy_status": model.InstallDeploySuccess,
		"pending_op":    "",
		"message":       "",
		"status":        helmStatus,
	}); err != nil {
		klog.Errorf("failed to mark appstore install(%d) success: %v", rec.Id, err)
	}

	// 仅「安装」成功计入 chart 累计安装次数（upgrade/rollback 不计，卸载不减）。
	// 计数为附属统计，失败只记日志，不影响本次安装结果。
	if op == model.InstallOpInstall {
		if err := c.factory.Chart().IncrementInstallCount(ctx, rec.ChartId); err != nil {
			klog.Errorf("failed to increment install count for chart(%d): %v", rec.ChartId, err)
		}
	}
	sink.Printf("操作完成")
}

// doInstall 执行安装：materialize → 安全预检 → helm install。
func (c *controller) doInstall(ctx context.Context, rec *model.AppstoreInstall, sink *logSink) (string, error) {
	cv, err := c.factory.Chart().GetChartVersion(ctx, rec.ChartId, rec.Version)
	if err != nil {
		return "", err
	}
	if cv == nil {
		return "", fmt.Errorf("chart version %s not found", rec.Version)
	}

	sink.Printf("解析 chart 包: %s", cv.StorageKey)
	path, cleanup, err := c.materialize(ctx, cv.StorageKey)
	if err != nil {
		return "", err
	}
	defer cleanup()

	sink.Printf("安全预检 ...")
	if err = c.guard(path, rec.ReleaseName, rec.Namespace, valuesMap(rec.Values), c.isAdminUser(ctx, rec.CreatedBy)); err != nil {
		return "", err
	}

	sink.Printf("下发集群资源 (cluster=%s namespace=%s)", rec.Cluster, rec.Namespace)
	rel, err := c.helmSvc.Release(rec.Cluster, rec.Namespace).Install(ctx, &types.Release{
		Name:    rec.ReleaseName,
		Chart:   path,
		Version: cv.Version,
		Values:  valuesMap(rec.Values),
	})
	if err != nil {
		return "", fmt.Errorf("failed to install chart: %w", err)
	}
	status := releaseStatus(rel)
	sink.Printf("helm release: %s", status)
	return status, nil
}

// doUpgrade 执行升级：materialize 目标版本 → 安全预检 → helm upgrade。
func (c *controller) doUpgrade(ctx context.Context, rec *model.AppstoreInstall, sink *logSink) (string, error) {
	cv, err := c.factory.Chart().GetChartVersion(ctx, rec.ChartId, rec.Version)
	if err != nil {
		return "", err
	}
	if cv == nil {
		return "", fmt.Errorf("chart version %s not found", rec.Version)
	}

	sink.Printf("解析目标 chart 包: %s", cv.StorageKey)
	path, cleanup, err := c.materialize(ctx, cv.StorageKey)
	if err != nil {
		return "", err
	}
	defer cleanup()

	sink.Printf("安全预检 ...")
	if err = c.guard(path, rec.ReleaseName, rec.Namespace, valuesMap(rec.Values), c.isAdminUser(ctx, rec.CreatedBy)); err != nil {
		return "", err
	}

	sink.Printf("升级 release 至 %s", cv.Version)
	rel, err := c.helmSvc.Release(rec.Cluster, rec.Namespace).Upgrade(ctx, &types.Release{
		Name:    rec.ReleaseName,
		Chart:   path,
		Version: cv.Version,
		Values:  valuesMap(rec.Values),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upgrade release: %w", err)
	}
	status := releaseStatus(rel)
	sink.Printf("helm release: %s", status)
	return status, nil
}

// doRollback 执行回滚：helm rollback 到目标 revision，随后回读 release 状态。
func (c *controller) doRollback(ctx context.Context, rec *model.AppstoreInstall, sink *logSink) (string, error) {
	sink.Printf("回滚 release %s 至 revision %d", rec.ReleaseName, rec.TargetRevision)
	relSvc := c.helmSvc.Release(rec.Cluster, rec.Namespace)
	if err := relSvc.Rollback(ctx, rec.ReleaseName, rec.TargetRevision); err != nil {
		return "", fmt.Errorf("failed to rollback release: %w", err)
	}

	status := ""
	if rel, err := relSvc.Get(ctx, rec.ReleaseName); err == nil && rel != nil {
		status = releaseStatus(rel)
	}
	sink.Printf("helm release: %s", status)
	return status, nil
}

// doUninstall 执行卸载：集群内存在该 release 时先 helm uninstall，再删除记账记录。
// 若集群内已无该 release（例如安装失败遗留记录），直接清理记账 —— 这也是「删除失败记录」的落地路径。
func (c *controller) doUninstall(ctx context.Context, rec *model.AppstoreInstall, sink *logSink) error {
	relSvc := c.helmSvc.Release(rec.Cluster, rec.Namespace)
	if rel, err := relSvc.Get(ctx, rec.ReleaseName); err == nil && rel != nil {
		sink.Printf("卸载 release %s", rec.ReleaseName)
		if _, err = relSvc.Uninstall(ctx, rec.ReleaseName); err != nil {
			return fmt.Errorf("failed to uninstall release: %w", err)
		}
	} else {
		sink.Printf("集群内未找到 release，直接清理记账")
	}

	if err := c.factory.Chart().DeleteInstall(ctx, rec.Id); err != nil {
		return fmt.Errorf("failed to delete install record: %w", err)
	}
	return nil
}

// isAdminUser 依据记录的 created_by 用户 role 判定是否管理员（worker 无请求用户 ctx）。
func (c *controller) isAdminUser(ctx context.Context, userID int64) bool {
	user, err := c.factory.User().Get(ctx, userID)
	if err != nil {
		klog.Errorf("failed to get creator(%d) for appstore install: %v", userID, err)
		return false
	}
	return isAdmin(user)
}

// valuesMap 把记录里的 values JSON 文本解析为 map；空串/非法 JSON 返回 nil。
func valuesMap(raw string) map[string]interface{} {
	if raw == "" {
		return nil
	}
	m := make(map[string]interface{})
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

// opLabel 操作类型的中文展示名。
func opLabel(op string) string {
	switch op {
	case model.InstallOpInstall:
		return "安装"
	case model.InstallOpUpgrade:
		return "升级"
	case model.InstallOpRollback:
		return "回滚"
	case model.InstallOpUninstall:
		return "卸载"
	}
	return op
}

// ---- 日志 sink ----

// logSink 是 per-install 的日志落盘器：worker 边执行边写，SSE 端点再 tail 该文件回放/追流。
// helm 内部 klog 噪音不采集，只记录编排进度行。
type logSink struct {
	mu sync.Mutex
	f  *os.File
}

// logDir 应用商店日志目录：<appstore.data_dir>/logs。
func (c *controller) logDir() string {
	dir := c.cc.Appstore.DataDir
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "pixiu-appstore")
	}
	return filepath.Join(dir, "logs")
}

// logPath 单条 install 的日志文件路径。
func (c *controller) logPath(id int64) string {
	return filepath.Join(c.logDir(), fmt.Sprintf("install-%d.log", id))
}

// newLogSink 打开（截断重写）本次执行的日志文件；打开失败时返回空 sink（写日志降级为 no-op）。
func (c *controller) newLogSink(id int64) *logSink {
	dir := c.logDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		klog.Errorf("failed to create appstore log dir %s: %v", dir, err)
		return &logSink{}
	}
	f, err := os.OpenFile(c.logPath(id), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		klog.Errorf("failed to open appstore log file for install(%d): %v", id, err)
		return &logSink{}
	}
	return &logSink{f: f}
}

func (s *logSink) Printf(format string, args ...interface{}) {
	if s == nil || s.f == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(s.f, "[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
}

func (s *logSink) close() {
	if s == nil || s.f == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.f.Close()
	s.f = nil
}

// WatchLogs SSE 实时推送单条安装的执行日志：先回放已生成内容，再 follow 新增（tail），客户端断开即退出。
// 以集群作用域自然身份 (cluster, namespace, name) 定位平台记账记录；只有平台记账的 release 才有执行日志，
// 未命中记录（含外部 release）→ 404。
func (c *controller) WatchLogs(ctx context.Context, cluster, namespace, name string, w http.ResponseWriter, r *http.Request) error {
	_, rec, _, err := c.authorizeAndLoadMaybe(ctx, cluster, namespace, name)
	if err != nil {
		return err
	}
	if rec == nil {
		return apierrors.NewError(fmt.Errorf("该 release 无平台执行日志"), http.StatusNotFound)
	}
	id := rec.Id

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flush, _ := w.(http.Flusher)

	path := c.logPath(id)
	var offset int64
	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-ctx.Done():
			return nil
		default:
		}

		next, err := tailFile(w, path, offset)
		if err != nil {
			return nil
		}
		if next != offset {
			offset = next
			if flush != nil {
				flush.Flush()
			}
		}
		time.Sleep(logPollInterval)
	}
}

// tailFile 从 offset 处把文件新增内容拷到 w，返回新的偏移量。
// 文件不存在返回原偏移；文件被截断重写（size < offset）时从头读取。
func tailFile(w io.Writer, path string, offset int64) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return offset, nil
		}
		return offset, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return offset, err
	}
	if fi.Size() < offset {
		offset = 0
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return offset, err
	}
	n, err := io.Copy(w, f)
	if err != nil {
		return offset, err
	}
	return offset + n, nil
}
