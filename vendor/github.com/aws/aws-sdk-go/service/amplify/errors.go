// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

package amplify

import (
	"github.com/aws/aws-sdk-go/private/protocol"
)

const (

	// ErrCodeBadRequestException for service response error code
	// "BadRequestException".
	//
	// Exception thrown when a request contains unexpected data.
	ErrCodeBadRequestException = "BadRequestException"

	// ErrCodeDependentServiceFailureException for service response error code
	// "DependentServiceFailureException".
	//
	// Exception thrown when an operation fails due to a dependent service throwing
	// an exception.
	ErrCodeDependentServiceFailureException = "DependentServiceFailureException"

	// ErrCodeInternalFailureException for service response error code
	// "InternalFailureException".
	//
	// Exception thrown when the service fails to perform an operation due to an
	// internal issue.
	ErrCodeInternalFailureException = "InternalFailureException"

	// ErrCodeLimitExceededException for service response error code
	// "LimitExceededException".
	//
	// Exception thrown when a resource could not be created because of service
	// limits.
	ErrCodeLimitExceededException = "LimitExceededException"

	// ErrCodeNotFoundException for service response error code
	// "NotFoundException".
	//
	// Exception thrown when an entity has not been found during an operation.
	ErrCodeNotFoundException = "NotFoundException"

	// ErrCodeResourceNotFoundException for service response error code
	// "ResourceNotFoundException".
	//
	// Exception thrown when an operation fails due to non-existent resource.
	ErrCodeResourceNotFoundException = "ResourceNotFoundException"

	// ErrCodeUnauthorizedException for service response error code
	// "UnauthorizedException".
	//
	// Exception thrown when an operation fails due to a lack of access.
	ErrCodeUnauthorizedException = "UnauthorizedException"
)

var exceptionFromCode = map[string]func(protocol.ResponseMetadata) error{
	"BadRequestException":              newErrorBadRequestException,
	"DependentServiceFailureException": newErrorDependentServiceFailureException,
	"InternalFailureException":         newErrorInternalFailureException,
	"LimitExceededException":           newErrorLimitExceededException,
	"NotFoundException":                newErrorNotFoundException,
	"ResourceNotFoundException":        newErrorResourceNotFoundException,
	"UnauthorizedException":            newErrorUnauthorizedException,
}