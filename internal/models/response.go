package models

// ErrorDetail contains structured error information.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse is the standardized error response envelope.
type ErrorResponse struct {
	Success bool        `json:"success,omitempty"`
	Error   ErrorDetail `json:"error"`
}

// NewErrorResponse constructs a standardized ErrorResponse.
func NewErrorResponse(code, message string) ErrorResponse {
	return ErrorResponse{
		Success: false,
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
}

// SuccessResponse is the standardized success envelope for v1 APIs.
type SuccessResponse struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
}

// NewSuccessResponse constructs a standardized SuccessResponse.
func NewSuccessResponse(data any) SuccessResponse {
	return SuccessResponse{
		Success: true,
		Data:    data,
	}
}
