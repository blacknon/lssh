//go:build !cgo && (darwin || linux)

package onepassword

// WithDesktopAppIntegration specifies a client should use the desktop app to authenticate. Set to your 1Password account name as shown at the top left sidebar of the app, or your account UUID.
func WithDesktopAppIntegration(accountName string) ClientOption {
	var _ = ERROR_WithDesktopAppIntegration_requires_CGO_To_Cross_Compile_See_README_CGO_Section
	return nil
}
