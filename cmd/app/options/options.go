/*
Copyright 2021 The Pixiu Authors.

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

package options

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	pixiudb "github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/jobmanager"
	"github.com/caoyingjunz/pixiu/pkg/tunnel"
	pixiuruntime "github.com/caoyingjunz/pixiu/pkg/util/runtime"
	pixiuConfig "github.com/caoyingjunz/pixiulib/config"
)

const (
	maxIdleConns = 10
	maxOpenConns = 100

	defaultListen     = 8091
	defaultToolbox    = "crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu-toolbox:v2.0.1"
	defaultConfigFile = "/etc/pixiu/config.yaml"
	defaultLogFormat  = config.LogFormatJson
	defaultWorkDir    = "/etc/pixiu"
	defaultStaticDir  = "/static"

	// defaultDeployTimeout 本地部署任务容器等待超时(秒)，默认 30 分钟
	defaultDeployTimeout = 1800

	defaultAdminUser   = "admin"
	defaultSingleLogin = false

	defaultSlowSQLDuration = 1 * time.Second
)

// Options has all the params needed to run a pixiu
type Options struct {
	// The default values.
	ComponentConfig config.Config
	HttpEngine      *gin.Engine

	// 数据库接口
	db      *gorm.DB
	Factory pixiudb.ShareDaoFactory
	// 貔貅主控制接口
	Controller controller.PixiuInterface

	// ConfigFile is the location of the pixiu server's configuration file.
	ConfigFile string

	JobManager *jobmanager.Manager

	AlertEvaluator *jobmanager.AlertEvaluator
}

func NewOptions() (*Options, error) {
	return &Options{
		HttpEngine: gin.Default(), // 初始化默认 api 路由
		ConfigFile: defaultConfigFile,
	}, nil
}

// isUnsafeJWTKey 判定 jwt_key 是否为不安全取值：为空或使用公开仓库中的示例值（等同任何人可伪造管理员 JWT）
func isUnsafeJWTKey(key string) bool {
	return key == "" || key == "pixiu"
}

// Complete completes all the required options
func (o *Options) Complete(cmd *cobra.Command) error {
	// 配置文件优先级: 默认配置，环境变量，命令行
	if len(o.ConfigFile) == 0 {
		// Try to read config file path from env.
		if cfgFile := os.Getenv("ConfigFile"); cfgFile != "" {
			o.ConfigFile = cfgFile
		} else {
			o.ConfigFile = defaultConfigFile
		}
	}

	c := pixiuConfig.New()
	c.SetConfigFile(o.ConfigFile)
	c.SetConfigType("yaml")
	if err := c.Binding(&o.ComponentConfig); err != nil {
		return err
	}

	// TODO: move to config initialization?
	if o.ComponentConfig.Default.Listen == 0 {
		o.ComponentConfig.Default.Listen = defaultListen
	}
	// jwt_key 安全校验：不再注入仓库硬编码默认值（"pixiu" 公开可见，等同于任何人可伪造管理员 JWT）。
	// release 模式（含未配置 mode）下为空或仍为示例值时拒绝启动；debug 模式仅限本地开发，允许但显式告警。
	if isUnsafeJWTKey(o.ComponentConfig.Default.JWTKey) {
		if o.ComponentConfig.Default.Mode.InDebug() {
			klog.Warningf("jwt_key 为空或使用了公开示例值，仅可用于本地开发；请在配置中设置强随机 jwt_key（如 openssl rand -hex 32 生成）")
		} else {
			return fmt.Errorf("jwt_key 为空或使用了公开示例值，拒绝启动；请在配置中设置强随机 jwt_key（如 openssl rand -hex 32 生成）")
		}
	}
	if len(o.ComponentConfig.Default.Toolbox) == 0 {
		o.ComponentConfig.Default.Toolbox = defaultToolbox
	}
	if o.ComponentConfig.Log.LogFormat == "" {
		o.ComponentConfig.Log.LogFormat = defaultLogFormat
	}
	if o.ComponentConfig.Worker.WorkDir == "" {
		o.ComponentConfig.Worker.WorkDir = defaultWorkDir
	}
	if o.ComponentConfig.Worker.DeployTimeout <= 0 {
		o.ComponentConfig.Worker.DeployTimeout = defaultDeployTimeout
	}
	if len(o.ComponentConfig.Default.StaticFiles) == 0 {
		o.ComponentConfig.Default.StaticFiles = defaultStaticDir
	}
	if o.ComponentConfig.Audit.Schedule == "" {
		o.ComponentConfig.Audit.Schedule = jobmanager.DefaultSchedule
	}
	if o.ComponentConfig.Audit.DaysReserved == 0 {
		o.ComponentConfig.Audit.DaysReserved = jobmanager.DefaultDaysReserved
	}
	if o.ComponentConfig.AlertHistory.Schedule == "" {
		o.ComponentConfig.AlertHistory.Schedule = jobmanager.DefaultAlertHistorySchedule
	}
	if o.ComponentConfig.AlertHistory.DaysReserved == 0 {
		o.ComponentConfig.AlertHistory.DaysReserved = jobmanager.DefaultAlertHistoryDaysReserved
	}
	if o.ComponentConfig.CronHpaHistory.Schedule == "" {
		o.ComponentConfig.CronHpaHistory.Schedule = jobmanager.DefaultCronHpaHistorySchedule
	}
	if o.ComponentConfig.CronHpaHistory.DaysReserved == 0 {
		o.ComponentConfig.CronHpaHistory.DaysReserved = jobmanager.DefaultCronHpaHistoryDaysReserved
	}
	if len(o.ComponentConfig.Default.AdminUser) == 0 {
		o.ComponentConfig.Default.AdminUser = defaultAdminUser
	}
	// admin_password 不再注入仓库默认口令：留空时由 bootstrapRootUser 生成一次性随机强密码并打印到启动日志

	if err := o.ComponentConfig.Valid(); err != nil {
		return err
	}

	// 登记宿主容器运行时配置（懒加载：启动不连接 CRI socket，
	// 首次部署/日志等实际使用时才建立连接，连接失败只影响当次调用，不阻断启动）
	pixiuruntime.Init(o.ComponentConfig.Runtime)

	o.ComponentConfig.Log.Init(isCLIVerbositySet(cmd))

	// 注册依赖组件
	if err := o.register(); err != nil {
		return err
	}

	o.Controller = controller.New(o.ComponentConfig, o.Factory)

	// 初始化 Agent 反向隧道（必须在路由注册前）
	tunnel.Init(tunnel.FactoryLookup{Factory: o.Factory})

	// 启动阶段一初始化：内置租户/角色/root 与目录类数据（路由安装前）
	if err := o.BootstrapPreRouter(); err != nil {
		return err
	}

	o.AlertEvaluator = jobmanager.NewAlertEvaluator(o.Factory)
	accessOpts := o.ComponentConfig.Log.AccessOptions()
	o.JobManager = jobmanager.NewManager(
		&accessOpts,
		jobmanager.NewAuditsCleaner(o.ComponentConfig.Audit, o.Factory),
		jobmanager.NewAlertHistoryCleaner(o.ComponentConfig.AlertHistory, o.Factory),
		jobmanager.NewCronHpaHistoryCleaner(o.ComponentConfig.CronHpaHistory, o.Factory),
		jobmanager.NewClusterSyncer(o.Factory, o.ComponentConfig.Default.Mode.InDebug()),
		jobmanager.NewAgentSyncer(o.Factory),
		jobmanager.NewTunnelSyncer(o.Factory),
		jobmanager.NewCronHpaEvaluator(o.Controller.Extension().Autoscaling()),
		o.AlertEvaluator,
	)
	return nil
}

// isCLIVerbositySet reports whether -v was explicitly provided on the command line.
func isCLIVerbositySet(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	f := cmd.Flags().Lookup("v")
	return f != nil && f.Changed
}

// BindFlags binds the pixiu Configuration struct fields
func (o *Options) BindFlags(cmd *cobra.Command) {
	// Expose klog flags (e.g. -v) to cobra. InitFlags registers them on
	// the standard flag.CommandLine in main; cobra will not accept them unless
	// they are added to the command flag set.
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	cmd.Flags().StringVar(&o.ConfigFile, "configfile", defaultConfigFile, "The location of the pixiu configuration file")
}

func (o *Options) register() error {
	// 注册数据库
	if err := o.registerDatabase(); err != nil {
		return err
	}

	return nil
}

func (o *Options) registerDatabase() error {
	sqlConfig := o.ComponentConfig.Mysql
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local",
		sqlConfig.User,
		sqlConfig.Password,
		sqlConfig.Host,
		sqlConfig.Port,
		sqlConfig.Name)

	opt := &gorm.Config{
		// SQL 仅通过 access log（log.sql）采集；不向 gorm 默认通道输出
		Logger: pixiudb.NewLogger(logger.Silent, defaultSlowSQLDuration),
	}
	db, err := gorm.Open(mysql.Open(dsn), opt)
	if err != nil {
		return err
	}
	o.db = db

	// 设置数据库连接池
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)

	o.Factory, err = pixiudb.NewDaoFactory(db, o.ComponentConfig.Default.AutoMigrate)
	return err
}

// Validate validates all the required options.
func (o *Options) Validate() error {
	// TODO
	return nil
}
