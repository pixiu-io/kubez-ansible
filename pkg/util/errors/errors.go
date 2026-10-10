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

package errors

import (
	"errors"

	"gorm.io/gorm"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrRecordNotFound              = gorm.ErrRecordNotFound
	ErrRecordNotUpdate             = errors.New("record not updated")
	ErrBusySystem                  = errors.New("系统繁忙，请稍后再试")
	ErrTooManyLoginAttempts        = errors.New("登录尝试过于频繁，请稍后再试")
	ErrTooManyRegistrationAttempts = errors.New("注册操作过于频繁，请稍后再试")
	ErrReqParams                   = errors.New("请求参数错误")
	ErrCloudNotRegister            = errors.New("cloud 集群未注册")
	ErrUserNotFound                = errors.New("用户不存在")
	ErrNotAcceptable               = errors.New("有任务正在执行，请稍后再试")
	ErrClusterNotFound             = errors.New("集群不存在")
	ErrPermissionNotFound          = errors.New("权限不存在")
	ErrUserPassword                = errors.New("密码错误")
	ErrInternal                    = errors.New("服务器内部错误")
	ErrTenantNotFound              = errors.New("租户不存在")
	ErrDuplicatedPassword          = errors.New("新密码与旧密码相同")
	ErrAuditNotFound               = errors.New("审计记录不存在")
	ErrAgentNotFound               = errors.New("Agent 不存在")
	ErrRunnerNotFound              = errors.New("Runner 不存在")
	ErrNodeNotFound                = errors.New("节点不存在")
	ErrNodeNameExists              = errors.New("该部署计划下已存在同名主机")
	ErrNodeIPExists                = errors.New("主机 IP 已存在")
	ErrInvalidNodeName             = errors.New("主机名需符合 Linux 规范：1-63 位，小写字母/数字/中划线，且不能以中划线开头或结尾")
	ErrInvalidNodeIP               = errors.New("主机 IP 地址不合法")
	ErrConnectivityAdminOnly       = errors.New("未保存节点的连通性检测仅限超级管理员，请先保存节点后再检测")
	DistributionExistError         = errors.New("操作系统发行版已存在")
	ErrDistributionNotFound        = errors.New("操作系统发行版不存在")

	ParamsError                    = errors.New("参数错误")
	OperateFailed                  = errors.New("操作失败")
	NoPermission                   = errors.New("无权限")
	InnerError                     = errors.New("内部错误")
	NoUserIdError                  = errors.New("请登录")
	UserExistError                 = errors.New("用户已存在")
	EmailExistError                = errors.New("邮箱已被注册")
	RegistrationCodeInvalidError   = errors.New("验证码错误")
	RegistrationCodeExpiredError   = errors.New("验证码已过期，请重新获取")
	RegistrationCodeUsedError      = errors.New("验证码已使用，请重新获取")
	RegistrationCodeAttemptsError  = errors.New("验证码错误次数过多，请重新获取")
	RegistrationCodeFrequentError  = errors.New("验证码发送过于频繁，请稍后再试")
	RegistrationEmailNotConfigured = errors.New("系统邮件未配置，请联系管理员")
	RegistrationEmailUnavailable   = errors.New("系统邮件暂不可用，请联系管理员")
	RegistrationRoleUnavailable    = errors.New("请先由管理员创建普通用户角色")
	RegistrationRoleConflict       = errors.New("存在多个同名普通用户角色，请联系管理员处理")
	RoleExistError                 = errors.New("角色已存在")
	RoleNotExistError              = errors.New("角色不存在")
	APIExistError                  = errors.New("API已存在")
	APINotExistError               = errors.New("API不存在")
	TenantExistError               = errors.New("租户已存在")
	ErrAuditExists                 = errors.New("审计记录已存在")

	ErrRootAlreadyExists = errors.New("超级管理员已存在")

	ErrPlanNotFound = errors.New("部署计划不存在")
	ErrConflict     = errors.New("操作冲突")
)

func IsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func IsNotUpdated(err error) bool {
	return errors.Is(err, ErrRecordNotUpdate)
}

func IsUniqueConstraintError(err error) bool {
	mysqlErr, ok := err.(*mysql.MySQLError)
	if !ok {
		return false
	}

	// 数据库的 1062 错误码为固定的主键冲突号
	return mysqlErr.Number == 1062
}
