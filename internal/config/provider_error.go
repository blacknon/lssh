package conf

import "errors"

type ProviderError struct {
	Provider string
	Code     string
	Message  string
}

func (e *ProviderError) Error() string {
	if e == nil {
		return ""
	}
	return `provider "` + e.Provider + `": ` + e.Message
}

func IsProviderErrorCode(err error, code string) bool {
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		return false
	}
	return providerErr.Code == code
}
