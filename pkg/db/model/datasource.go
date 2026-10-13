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

package model

import "github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"

func init() {
	register(&Datasource{})
}

type DatasourceType int
type DatasourceSubType string

const (
	DatasourceTypeLog        DatasourceType = 0
	DatasourceTypeAlert      DatasourceType = 1
	DatasourceTypeRedis      DatasourceType = 2
	DatasourceTypeMiddleware DatasourceType = 3
)

const (
	DatasourceSubTypeLoki         DatasourceSubType = "loki"
	DatasourceSubTypeES           DatasourceSubType = "es"
	DatasourceSubTypeVictoriaLogs DatasourceSubType = "vmlogs"
	DatasourceSubTypePrometheus   DatasourceSubType = "prometheus"
	DatasourceSubTypeRedis        DatasourceSubType = "redis"
	DatasourceSubTypeNacos        DatasourceSubType = "nacos"
	DatasourceSubTypeMySQL        DatasourceSubType = "mysql"
	DatasourceSubTypeRabbitMQ     DatasourceSubType = "rabbitmq"
	DatasourceSubTypePostgres     DatasourceSubType = "postgres"
	DatasourceSubTypeStorage      DatasourceSubType = "storage"
)

type Datasource struct {
	pixiu.Model

	UserId      int64             `gorm:"index:idx_user_id" json:"user_id"` // 所属用户
	ClusterName string            `gorm:"column:cluster_name;type:varchar(128)" json:"cluster_name"`
	Name        string            `gorm:"column:name;type:varchar(128);not null" json:"name"`
	Type        DatasourceType    `gorm:"column:type;not null" json:"type"`
	SubType     DatasourceSubType `gorm:"column:sub_type;type:varchar(32);not null" json:"sub_type"`
	Config      string            `gorm:"column:config;type:text" json:"config"`
	IsDefault   bool              `gorm:"column:is_default;default:false;not null" json:"is_default"`
	External    bool              `gorm:"column:external;default:false;not null" json:"external"`
	Description string            `gorm:"column:description;type:text" json:"description"`
}

func (*Datasource) TableName() string {
	return "datasources"
}
