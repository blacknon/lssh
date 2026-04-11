# Security Policy

**Audience**: All users and implementers - security considerations and vulnerability reporting.

For implementation-specific security practices, see the [Implementer Guide](IMPLEMENTER_GUIDE.md#security-best-practices).

---

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| Latest  | :white_check_mark: |
| < Latest| :x:                |

## Reporting a Vulnerability

We take the security of absfs seriously. If you believe you have found a security vulnerability, please report it to us responsibly.

### How to Report

1. **DO NOT** open a public GitHub issue for security vulnerabilities
2. Email the maintainer at the address listed in CONTRIBUTORS
3. Include as much information as possible:
   - Type of vulnerability
   - Full path to affected source file(s)
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

### What to Expect

- Acknowledgment of your report within 48 hours
- Regular updates on the progress of addressing the vulnerability
- Credit in the CHANGELOG (unless you prefer to remain anonymous)

## Security Considerations

### Path Traversal Protection

absfs uses `filepath.Clean` to sanitize paths and prevent directory traversal attacks. All filesystem implementations should maintain this security boundary.

### Permission Constants Bug (FIXED)

**CVE**: Pending
**Severity**: HIGH
**Status**: FIXED in commit 5bce302

The `OS_ALL_RWX` constant was incorrectly defined as `OS_ALL_RW | OS_GROUP_X` instead of `OS_ALL_RW | OS_ALL_X`. This could lead to incorrect file permissions where execute permission was only granted to the group instead of all users. This has been fixed and verified with comprehensive tests.

**Impact**: Files created with `OS_ALL_RWX` (0777) would actually receive permissions 0776, potentially causing permission denied errors for others trying to execute the file.

**Mitigation**: Upgrade to the latest version. If upgrading is not immediately possible, avoid using `OS_ALL_RWX` and use numeric permission values (0777) instead.

### Goroutine Safety

**FileSystem instances are NOT goroutine-safe by default.** Each FileSystem object maintains its own current working directory (`cwd`), which can be modified by `Chdir`. Concurrent calls to `Chdir` and other path-dependent methods from multiple goroutines on the same FileSystem instance can cause race conditions.

**Best Practices:**
- Create separate FileSystem instances for each goroutine
- Use absolute paths exclusively to avoid cwd dependency
- If sharing a FileSystem across goroutines, use external synchronization (mutex)
- Consider implementing a goroutine-safe wrapper if needed

### Secret Management

When committing code or creating filesystems:
- Never commit files with secrets (.env, credentials.json, etc.)
- Use .gitignore to exclude sensitive files
- Filesystem implementations should not log file contents

## Known Limitations

1. **No builtin encryption** - absfs does not provide encryption; use OS-level encryption
2. **Permissions are advisory** - Actual enforcement depends on the underlying implementation
3. **No audit logging** - Filesystem operations are not logged by default

## Security Updates

Security updates will be announced via:
- GitHub Security Advisories
- CHANGELOG.md with [SECURITY] prefix
- Git tags for fixed versions

Stay informed by watching this repository.
