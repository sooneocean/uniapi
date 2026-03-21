package handler

// Common error messages used across handlers
const (
	errOperationFailed  = "operation failed"
	errNotFound         = "not found"
	errNotAuthenticated = "not authenticated"
	errAdminRequired    = "admin access required"
	errInvalidInput     = "invalid input"
	errTitleTooLong     = "title too long (max 500 chars)"
	errLabelTooLong     = "label too long (max 100 chars)"
	errUsernameTooLong  = "username too long (max 100 chars)"
	errContentTooLarge  = "content too large"
	errPasswordTooShort = "password must be at least 8 characters"
)
