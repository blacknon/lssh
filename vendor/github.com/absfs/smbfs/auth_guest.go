package smbfs

// Authenticator represents an authentication mechanism for SMB sessions
type Authenticator interface {
	// Authenticate processes an authentication request and returns the result
	// securityBlob is the security buffer from the SESSION_SETUP request
	Authenticate(securityBlob []byte) (*AuthResult, error)
}

// AuthResult contains the result of an authentication attempt
type AuthResult struct {
	Success    bool   // Whether authentication succeeded
	IsGuest    bool   // Whether this is a guest session
	Username   string // Authenticated username (empty for guest)
	Domain     string // User's domain (empty for guest)
	SessionKey []byte // Session signing key (nil for guest/unsigned)

	// For multi-stage auth (like NTLM), this contains the security blob to return
	// If nil and Success=false, authentication is complete but failed
	// If non-nil and Success=false, more processing is required (STATUS_MORE_PROCESSING_REQUIRED)
	ResponseBlob []byte
}

// GuestAuthenticator implements guest-only authentication
// This is a simple authenticator that always succeeds with guest access
type GuestAuthenticator struct{}

// NewGuestAuthenticator creates a new guest authenticator
func NewGuestAuthenticator() *GuestAuthenticator {
	return &GuestAuthenticator{}
}

// Authenticate always succeeds with guest access
// In a real implementation, this would check if guest access is allowed
func (a *GuestAuthenticator) Authenticate(securityBlob []byte) (*AuthResult, error) {
	// Guest authentication always succeeds immediately
	return &AuthResult{
		Success:      true,
		IsGuest:      true,
		Username:     "Guest",
		Domain:       "",
		SessionKey:   nil, // No signing for guest sessions
		ResponseBlob: nil, // No security response needed
	}, nil
}
