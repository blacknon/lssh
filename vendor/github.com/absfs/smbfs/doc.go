// Package smbfs provides an SMB/CIFS network filesystem implementation
// for absfs - access Windows file shares and Samba servers through the
// absfs.FileSystem interface.
//
// # Overview
//
// smbfs enables seamless access to SMB network shares with full support
// for the absfs.FileSystem interface. It supports SMB2, SMB3, and SMB3.1.1
// protocols with enterprise-grade authentication including NTLM, Kerberos,
// and domain authentication.
//
// # Features
//
//   - Full absfs.FileSystem interface compliance
//   - SMB2, SMB3, and SMB3.1.1 protocol support
//   - Multiple authentication methods (NTLM, Kerberos, domain, guest)
//   - Connection pooling and session management
//   - Cross-platform SMB client (Windows, Linux, macOS)
//   - Large file support (>4GB)
//   - Composable with other absfs implementations
//
// # Basic Usage
//
// Connect to an SMB share with username/password:
//
//	fs, err := smbfs.New(&smbfs.Config{
//	    Server:   "fileserver.example.com",
//	    Share:    "shared",
//	    Username: "jdoe",
//	    Password: "secret123",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer fs.Close()
//
//	// Use standard file operations
//	data, err := fs.ReadFile("/path/to/file.txt")
//
// # Connection String
//
// Alternatively, use a connection string:
//
//	fs, err := smbfs.ParseConnectionString("smb://user:pass@server/share")
//
// # Authentication Methods
//
// Username/Password (NTLM):
//
//	&smbfs.Config{
//	    Server:   "fileserver.example.com",
//	    Share:    "shared",
//	    Username: "jdoe",
//	    Password: "secret123",
//	    Domain:   "CORP",  // Optional for domain-joined servers
//	}
//
// Kerberos Authentication:
//
//	&smbfs.Config{
//	    Server:      "fileserver.corp.example.com",
//	    Share:       "departments",
//	    UseKerberos: true,
//	    Domain:      "CORP",
//	    Username:    "jdoe",
//	}
//
// Guest Access:
//
//	&smbfs.Config{
//	    Server:      "public.example.com",
//	    Share:       "public",
//	    GuestAccess: true,
//	}
//
// # Configuration
//
// The Config structure provides extensive customization options:
//
//   - Server connection (server, port, share)
//   - Authentication (username, password, domain, Kerberos)
//   - Connection pooling (max idle/open, timeouts)
//   - Performance tuning (buffer sizes, caching)
//
// # Composition
//
// smbfs can be composed with other absfs implementations:
//
//	// Add caching for better performance
//	cached := cachefs.New(smbfs.New(...), memfs.New())
//
//	// Add metrics for monitoring
//	monitored := metricsfs.New(smbfs.New(...))
//
// # Platform Support
//
// smbfs works on all platforms supported by Go:
//
//   - Linux (native performance)
//   - macOS (native performance)
//   - Windows (alternative to built-in SMB)
//   - FreeBSD and other Unix systems
//
// Pure Go implementation with no CGO dependencies.
package smbfs
