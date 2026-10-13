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

// Package extension 提供扩展能力父接口，redis、mysql、autoscaling  等子模块挂在下面
package extension

import (
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	autoscalingcontroller "github.com/caoyingjunz/pixiu/pkg/controller/extension/autoscaling"
	mysqlcontroller "github.com/caoyingjunz/pixiu/pkg/controller/extension/mysql"
	postgrescontroller "github.com/caoyingjunz/pixiu/pkg/controller/extension/postgres"
	rediscontroller "github.com/caoyingjunz/pixiu/pkg/controller/extension/redis"
	storagecontroller "github.com/caoyingjunz/pixiu/pkg/controller/extension/storage"
	"github.com/caoyingjunz/pixiu/pkg/db"
)

type Getter interface {
	Extension() Interface
}

// Interface 扩展能力父接口，redis、mysql、autoscaling  等子模块挂在下面
type Interface interface {
	Redis() rediscontroller.Interface
	Autoscaling() autoscalingcontroller.Interface
	Mysql() mysqlcontroller.Interface
	Postgres() postgrescontroller.Interface
	Storage() storagecontroller.Interface
}

type controller struct {
	redis       rediscontroller.Interface
	autoscaling autoscalingcontroller.Interface
	mysql       mysqlcontroller.Interface
	postgres    postgrescontroller.Interface
	storage     storagecontroller.Interface
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{
		redis:       rediscontroller.New(cfg, f),
		autoscaling: autoscalingcontroller.New(cfg, f),
		mysql:       mysqlcontroller.New(cfg, f),
		postgres:    postgrescontroller.New(cfg, f),
		storage:     storagecontroller.New(cfg, f),
	}
}

func (c *controller) Redis() rediscontroller.Interface {
	return c.redis
}

func (c *controller) Autoscaling() autoscalingcontroller.Interface {
	return c.autoscaling
}

func (c *controller) Mysql() mysqlcontroller.Interface {
	return c.mysql
}

func (c *controller) Postgres() postgrescontroller.Interface { return c.postgres }

func (c *controller) Storage() storagecontroller.Interface { return c.storage }
