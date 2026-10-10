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

package errors

import (
	"net/http"

	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Error struct {
	Code int
	Err  error
}

func (e Error) Error() string {
	return e.Err.Error()
}

func NewError(err error, code int) Error {
	return Error{
		Code: code,
		Err:  err,
	}
}

var (
	ErrUnauthorized = Error{
		Code: http.StatusUnauthorized,
		Err:  errors.NoUserIdError,
	}
	ErrForbidden = Error{
		Code: http.StatusForbidden,
		Err:  errors.NoPermission,
	}
	ErrInvalidRequest = Error{
		Code: http.StatusBadRequest,
		Err:  errors.ErrReqParams,
	}
	ErrServerInternal = Error{
		Code: http.StatusInternalServerError,
		Err:  errors.ErrInternal,
	}
	ErrUserNotFound = Error{
		Code: http.StatusNotFound,
		Err:  errors.ErrUserNotFound,
	}
	ErrNotAcceptable = Error{
		Code: http.StatusNotAcceptable,
		Err:  errors.ErrNotAcceptable,
	}
	ErrUserExists = Error{
		Code: http.StatusConflict,
		Err:  errors.UserExistError,
	}
	ErrInvalidPassword = Error{
		Code: http.StatusUnauthorized,
		Err:  errors.ErrUserPassword,
	}
	ErrTooManyLoginAttempts = Error{
		Code: http.StatusTooManyRequests,
		Err:  errors.ErrTooManyLoginAttempts,
	}
	ErrTooManyRegistrationAttempts = Error{
		Code: http.StatusTooManyRequests,
		Err:  errors.ErrTooManyRegistrationAttempts,
	}
	ErrEmailExists = Error{
		Code: http.StatusConflict,
		Err:  errors.EmailExistError,
	}
	ErrRegistrationCodeInvalid = Error{
		Code: http.StatusBadRequest,
		Err:  errors.RegistrationCodeInvalidError,
	}
	ErrRegistrationCodeExpired = Error{
		Code: http.StatusBadRequest,
		Err:  errors.RegistrationCodeExpiredError,
	}
	ErrRegistrationCodeUsed = Error{
		Code: http.StatusBadRequest,
		Err:  errors.RegistrationCodeUsedError,
	}
	ErrRegistrationCodeAttempts = Error{
		Code: http.StatusTooManyRequests,
		Err:  errors.RegistrationCodeAttemptsError,
	}
	ErrRegistrationCodeTooFrequent = Error{
		Code: http.StatusTooManyRequests,
		Err:  errors.RegistrationCodeFrequentError,
	}
	ErrEmailNotConfigured = Error{
		Code: http.StatusServiceUnavailable,
		Err:  errors.RegistrationEmailNotConfigured,
	}
	ErrRegistrationEmailUnavailable = Error{
		Code: http.StatusServiceUnavailable,
		Err:  errors.RegistrationEmailUnavailable,
	}
	ErrRegistrationRoleUnavailable = Error{
		Code: http.StatusServiceUnavailable,
		Err:  errors.RegistrationRoleUnavailable,
	}
	ErrRegistrationRoleConflict = Error{
		Code: http.StatusConflict,
		Err:  errors.RegistrationRoleConflict,
	}
	ErrDuplicatedPassword = Error{
		Code: http.StatusConflict,
		Err:  errors.ErrDuplicatedPassword,
	}
	ErrClusterNotFound = Error{
		Code: http.StatusNotFound,
		Err:  errors.ErrClusterNotFound,
	}
	ErrTenantExists = Error{
		Code: http.StatusConflict,
		Err:  errors.TenantExistError,
	}
	ErrTenantNotFound = Error{
		Code: http.StatusNotFound,
		Err:  errors.ErrTenantNotFound,
	}
	ErrRoleExists = Error{
		Code: http.StatusConflict,
		Err:  errors.RoleExistError,
	}
	ErrRoleNotFound = Error{
		Code: http.StatusNotFound,
		Err:  errors.RoleNotExistError,
	}
	ErrAPIExists = Error{
		Code: http.StatusConflict,
		Err:  errors.APIExistError,
	}
	ErrAPINotFound = Error{
		Code: http.StatusNotFound,
		Err:  errors.APINotExistError,
	}
	ErrAuditNotFound = Error{
		Code: http.StatusNotFound,
		Err:  errors.ErrAuditNotFound,
	}
	ErrAuditExists = Error{
		Code: http.StatusConflict,
		Err:  errors.ErrAuditExists,
	}
	ErrAgentNotFound = Error{
		Code: http.StatusNotFound,
		Err:  errors.ErrAgentNotFound,
	}
	ErrRunnerNotFound = Error{
		Code: http.StatusNotFound,
		Err:  errors.ErrRunnerNotFound,
	}
	ErrNodeNotFound = Error{
		Code: http.StatusNotFound,
		Err:  errors.ErrNodeNotFound,
	}
	ErrNodeNameExists = Error{
		Code: http.StatusConflict,
		Err:  errors.ErrNodeNameExists,
	}
	ErrNodeIPExists = Error{
		Code: http.StatusConflict,
		Err:  errors.ErrNodeIPExists,
	}
	ErrInvalidNodeName = Error{
		Code: http.StatusBadRequest,
		Err:  errors.ErrInvalidNodeName,
	}
	ErrInvalidNodeIP = Error{
		Code: http.StatusBadRequest,
		Err:  errors.ErrInvalidNodeIP,
	}
	ErrConnectivityAdminOnly = Error{
		Code: http.StatusForbidden,
		Err:  errors.ErrConnectivityAdminOnly,
	}
	ErrDistributionExists = Error{
		Code: http.StatusConflict,
		Err:  errors.DistributionExistError,
	}
	ErrDistributionNotFound = Error{
		Code: http.StatusNotFound,
		Err:  errors.ErrDistributionNotFound,
	}
	ErrRootAlreadyExists = Error{
		Code: http.StatusConflict,
		Err:  errors.ErrRootAlreadyExists,
	}
	ErrPlanNotFound = Error{
		Code: http.StatusNotFound,
		Err:  errors.ErrPlanNotFound,
	}
	ErrConflict = Error{
		Code: http.StatusConflict,
		Err:  errors.ErrConflict,
	}
)
