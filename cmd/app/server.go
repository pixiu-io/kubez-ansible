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

package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/router"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/chartstore"
	"github.com/caoyingjunz/pixiu/pkg/util/tlscert"
)

func NewServerCommand(version string) *cobra.Command {
	opts, err := options.NewOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}

	cmd := &cobra.Command{
		Use:  "pixiu-server",
		Long: "The pixiu server controller is a daemon that embeds the core control loops.",
		Run: func(cmd *cobra.Command, args []string) {
			if err = opts.Complete(cmd); err != nil {
				klog.Fatal(err)
			}
			if err = opts.Validate(); err != nil {
				klog.Fatal(err)
			}
			if err = Run(opts); err != nil {
				klog.Fatal(err)
			}
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	// 绑定命令行参数
	opts.BindFlags(cmd)

	verCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Long:  "Print version and exit.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	}
	cmd.AddCommand(verCmd)
	return cmd
}

// Run 优雅启动貔貅服务
func Run(opt *options.Options) error {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", opt.ComponentConfig.Default.Listen),
		Handler: opt.HttpEngine,
	}

	// TODO: 暂未设置优雅退出
	// 启动集群相关控制器
	runers := []func(context.Context, int) error{opt.Controller.Plan().Run, opt.Controller.Cluster().Run, opt.Controller.Appstore().Run}
	for _, runner := range runers {
		if err := runner(context.TODO(), 5); err != nil {
			klog.Fatal("failed to start manager: ", err)
		}
	}

	// 启动阶段确保应用商店本地存储目录存在（storage=local 时），避免首次请求才建目录
	if err := chartstore.EnsureDataDir(opt.ComponentConfig.Appstore); err != nil {
		klog.Warningf("failed to ensure appstore data dir: %v", err)
	}

	// 安装 http 路由（同时将 API 持久化入库）
	router.InstallRouters(opt)
	// 启动阶段二初始化：初始化「普通用户」角色绑定 API/菜单，必须在路由安装后，依赖路由内的 api 初始化
	if err := opt.BootstrapPostRouter(context.Background()); err != nil {
		return err
	}

	var tlsSrv *http.Server
	if opt.ComponentConfig.TLS.IsEnabled() {
		certFile, keyFile, err := prepareTLSFiles(opt)
		if err != nil {
			return fmt.Errorf("prepare TLS certificates: %w", err)
		}
		tlsSrv = &http.Server{
			Addr:    fmt.Sprintf(":%d", opt.ComponentConfig.TLS.Listen),
			Handler: opt.HttpEngine,
		}
		go func() {
			klog.Infof("starting pixiu HTTPS server on :%d", opt.ComponentConfig.TLS.Listen)
			if err := tlsSrv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
				klog.Fatal("failed to listen pixiu HTTPS server: ", err)
			}
		}()
	}

	// Initializing the server in a goroutine so that it won't block the graceful shutdown handling below
	go func() {
		klog.Infof("starting pixiu HTTP server, listen on :%d", opt.ComponentConfig.Default.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Fatal("failed to listen pixiu server: ", err)
		}
	}()

	klog.Info("starting job manager")
	opt.JobManager.Run()

	// Wait for interrupt signal to gracefully shut down the server with a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	klog.Info("shutting pixiu server down ...")

	// The context is used to inform the server it has 5 seconds to finish the request
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		klog.Fatalf("pixiu server forced to shutdown: %v", err)
	}
	if tlsSrv != nil {
		if err := tlsSrv.Shutdown(ctx); err != nil {
			klog.Fatalf("pixiu HTTPS server forced to shutdown: %v", err)
		}
	}

	klog.Info("shutting job manager down ...")
	opt.JobManager.Stop()
	if opt.AlertEvaluator != nil {
		opt.AlertEvaluator.Stop()
	}

	return nil
}

func prepareTLSFiles(opt *options.Options) (certFile, keyFile string, err error) {
	tlsCfg := &opt.ComponentConfig.TLS
	tlsCfg.SetDefaults()
	certFile = tlsCfg.CertFile
	keyFile = tlsCfg.KeyFile
	if certFile == "" || keyFile == "" {
		dir := filepath.Join(opt.ComponentConfig.Worker.WorkDir, "certs")
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			dir = filepath.Join(".", "certs")
		}
		certFile = filepath.Join(dir, "server.crt")
		keyFile = filepath.Join(dir, "server.key")
		tlsCfg.CertFile = certFile
		tlsCfg.KeyFile = keyFile
	}
	hosts := tlsHostsFromPublicURL(opt.ComponentConfig.Default.PublicURL)
	if err = tlscert.EnsureSelfSigned(certFile, keyFile, hosts); err != nil {
		return "", "", err
	}
	return certFile, keyFile, nil
}

func tlsHostsFromPublicURL(publicURL string) []string {
	u, err := url.Parse(publicURL)
	if err != nil || u.Hostname() == "" {
		return nil
	}
	return []string{u.Hostname()}
}
