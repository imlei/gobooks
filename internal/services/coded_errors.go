package services

import (
	"errors"
	"net/http"
)

type CodedError interface {
	error
	Code() string
	HTTPStatus() int
}

type ServiceError struct {
	code       string
	message    string
	httpStatus int
}

func NewServiceError(code, message string, httpStatus int) *ServiceError {
	if httpStatus == 0 {
		httpStatus = http.StatusBadRequest
	}
	return &ServiceError{code: code, message: message, httpStatus: httpStatus}
}

func NewPostingError(code, message string, httpStatus int) *ServiceError {
	return NewServiceError(code, message, httpStatus)
}

func (e *ServiceError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *ServiceError) Code() string {
	if e == nil {
		return ""
	}
	return e.code
}

func (e *ServiceError) HTTPStatus() int {
	if e == nil || e.httpStatus == 0 {
		return http.StatusBadRequest
	}
	return e.httpStatus
}

func ErrorCode(err error) string {
	var coded CodedError
	if errors.As(err, &coded) {
		return coded.Code()
	}
	return ""
}

func ErrorHTTPStatus(err error, fallback int) int {
	var coded CodedError
	if errors.As(err, &coded) {
		return coded.HTTPStatus()
	}
	if fallback != 0 {
		return fallback
	}
	return http.StatusInternalServerError
}
