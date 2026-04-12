# Kerberos Authentication Guide

This guide explains how to use Kerberos authentication with smbfs.

## Overview

Kerberos provides secure, ticket-based authentication for SMB connections. It's the preferred authentication method in Active Directory environments because:

- **No password transmission** over the network
- **Mutual authentication** between client and server
- **Time-limited tickets** reduce security risk
- **Single sign-on** across multiple services

## Prerequisites

### System Requirements

1. **Kerberos client libraries** installed
   ```bash
   # Debian/Ubuntu
   sudo apt-get install krb5-user libkrb5-dev

   # Red Hat/CentOS
   sudo yum install krb5-workstation krb5-libs

   # macOS (built-in)
   # No additional installation needed
   ```

2. **Configured `/etc/krb5.conf`**
   ```ini
   [libdefaults]
       default_realm = CORP.EXAMPLE.COM
       dns_lookup_realm = true
       dns_lookup_kdc = true
       ticket_lifetime = 24h
       renew_lifetime = 7d
       forwardable = true

   [realms]
       CORP.EXAMPLE.COM = {
           kdc = dc1.corp.example.com
           kdc = dc2.corp.example.com
           admin_server = dc1.corp.example.com
           default_domain = corp.example.com
       }

   [domain_realm]
       .corp.example.com = CORP.EXAMPLE.COM
       corp.example.com = CORP.EXAMPLE.COM
   ```

3. **Time synchronization** (critical for Kerberos)
   ```bash
   # Install NTP
   sudo apt-get install ntp

   # Or use systemd-timesyncd
   sudo timedatectl set-ntp true

   # Verify time sync
   timedatectl status
   ```

## Getting a Kerberos Ticket

### Interactive Login

```bash
# Obtain a ticket for your user
kinit username@CORP.EXAMPLE.COM

# Verify the ticket
klist

# Output:
# Ticket cache: FILE:/tmp/krb5cc_1000
# Default principal: username@CORP.EXAMPLE.COM
#
# Valid starting       Expires              Service principal
# 11/23/25 10:00:00  11/23/25 20:00:00  krbtgt/CORP.EXAMPLE.COM@CORP.EXAMPLE.COM
```

### Using a Keytab File

For service accounts or automated processes:

```bash
# Create a keytab (requires admin rights on AD)
ktutil
ktutil: addent -password -p username@CORP.EXAMPLE.COM -k 1 -e aes256-cts-hmac-sha1-96
Password for username@CORP.EXAMPLE.COM:
ktutil: wkt /etc/myapp.keytab
ktutil: quit

# Use the keytab
kinit -kt /etc/myapp.keytab username@CORP.EXAMPLE.COM

# Verify
klist
```

## Using Kerberos with smbfs

### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/absfs/smbfs"
)

func main() {
    // Ensure you have a valid Kerberos ticket first
    // Run: kinit username@CORP.EXAMPLE.COM

    fsys, err := smbfs.New(&smbfs.Config{
        Server:      "fileserver.corp.example.com",
        Share:       "departments",
        UseKerberos: true,                        // Enable Kerberos
        Domain:      "CORP.EXAMPLE.COM",          // Kerberos realm
        Username:    os.Getenv("USER"),           // Your username
        // Password is NOT needed - uses Kerberos ticket
    })
    if err != nil {
        log.Fatal(err)
    }
    defer fsys.Close()

    // Use the filesystem normally
    entries, err := fsys.ReadDir("/")
    if err != nil {
        log.Fatal(err)
    }

    for _, entry := range entries {
        fmt.Println(entry.Name())
    }
}
```

### With Keytab File

```go
package main

import (
    "log"
    "os"
    "os/exec"

    "github.com/absfs/smbfs"
)

func main() {
    // Get ticket using keytab
    cmd := exec.Command("kinit", "-kt", "/etc/myapp.keytab", "service@CORP.EXAMPLE.COM")
    if err := cmd.Run(); err != nil {
        log.Fatal("Failed to get Kerberos ticket:", err)
    }

    // Now use Kerberos authentication
    fsys, err := smbfs.New(&smbfs.Config{
        Server:      "fileserver.corp.example.com",
        Share:       "shared",
        UseKerberos: true,
        Domain:      "CORP.EXAMPLE.COM",
        Username:    "service",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer fsys.Close()

    // Your code here...
}
```

### Automatic Ticket Renewal

For long-running services:

```go
package main

import (
    "context"
    "log"
    "os/exec"
    "time"

    "github.com/absfs/smbfs"
)

func renewTicket(keytab, principal string) error {
    cmd := exec.Command("kinit", "-kt", keytab, principal)
    return cmd.Run()
}

func startTicketRenewal(ctx context.Context, keytab, principal string) {
    ticker := time.NewTicker(4 * time.Hour) // Renew every 4 hours
    defer ticker.Stop()

    go func() {
        for {
            select {
            case <-ticker.C:
                if err := renewTicket(keytab, principal); err != nil {
                    log.Printf("Failed to renew Kerberos ticket: %v", err)
                } else {
                    log.Println("Kerberos ticket renewed successfully")
                }
            case <-ctx.Done():
                return
            }
        }
    }()
}

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    keytab := "/etc/myapp.keytab"
    principal := "service@CORP.EXAMPLE.COM"

    // Get initial ticket
    if err := renewTicket(keytab, principal); err != nil {
        log.Fatal("Failed to get initial Kerberos ticket:", err)
    }

    // Start automatic renewal
    startTicketRenewal(ctx, keytab, principal)

    // Create filesystem with Kerberos
    fsys, err := smbfs.New(&smbfs.Config{
        Server:      "fileserver.corp.example.com",
        Share:       "shared",
        UseKerberos: true,
        Domain:      "CORP.EXAMPLE.COM",
        Username:    "service",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer fsys.Close()

    // Long-running service code...
    select {}
}
```

## Troubleshooting

### "kinit: Cannot contact any KDC"

**Cause:** Cannot reach the Kerberos KDC (Domain Controller)

**Solutions:**
```bash
# Check network connectivity
ping dc1.corp.example.com

# Check if KDC is reachable on port 88
nc -zv dc1.corp.example.com 88

# Verify DNS resolution
nslookup dc1.corp.example.com

# Check krb5.conf syntax
grep -v '^#' /etc/krb5.conf | grep -v '^$'
```

### "kinit: Clock skew too great"

**Cause:** Local time differs too much from KDC time (usually >5 minutes)

**Solutions:**
```bash
# Check time difference
ssh dc1.corp.example.com date

# Sync time immediately
sudo ntpdate -s dc1.corp.example.com
# or
sudo chronyd -q 'server dc1.corp.example.com iburst'

# Enable automatic time sync
sudo timedatectl set-ntp true
```

### "kinit: Preauthentication failed"

**Cause:** Invalid password or principal doesn't exist

**Solutions:**
```bash
# Verify principal exists in AD
# (from Windows DC)
Get-ADUser -Identity username

# Try with full realm
kinit username@CORP.EXAMPLE.COM

# Check for typos in realm name (case-sensitive)
```

### "Failed to mount share: authentication failed"

**Cause:** Kerberos ticket not being used or expired

**Solutions:**
```bash
# Check if ticket exists and is valid
klist

# Renew ticket
kinit -R
# or get new ticket
kinit username@CORP.EXAMPLE.COM

# Verify ticket is for correct realm
klist | grep 'Default principal'

# Check if ticket has required service principal
klist | grep -i cifs
```

### "Cannot find KDC for realm"

**Cause:** DNS SRV records not configured or dns_lookup_kdc disabled

**Solutions:**
```bash
# Test DNS SRV records
dig _kerberos._tcp.corp.example.com SRV

# Enable DNS lookup in /etc/krb5.conf
[libdefaults]
    dns_lookup_kdc = true
    dns_lookup_realm = true

# Or explicitly list KDCs in [realms] section
```

## Best Practices

### Security

1. **Use keytab files for service accounts**
   - More secure than embedded passwords
   - Enable automatic authentication
   - Protect keytab files (chmod 600)

2. **Set appropriate ticket lifetimes**
   ```ini
   [libdefaults]
       ticket_lifetime = 10h        # Work day
       renew_lifetime = 7d          # Week
   ```

3. **Enable forwardable tickets** for multi-hop scenarios
   ```ini
   [libdefaults]
       forwardable = true
   ```

4. **Use AES encryption** (disable weak algorithms)
   ```ini
   [libdefaults]
       default_tgs_enctypes = aes256-cts-hmac-sha1-96 aes128-cts-hmac-sha1-96
       default_tkt_enctypes = aes256-cts-hmac-sha1-96 aes128-cts-hmac-sha1-96
       permitted_enctypes = aes256-cts-hmac-sha1-96 aes128-cts-hmac-sha1-96
   ```

### Operations

1. **Monitor ticket expiration**
   ```go
   func checkTicketExpiration() (time.Duration, error) {
       cmd := exec.Command("klist")
       output, err := cmd.Output()
       if err != nil {
           return 0, err
       }
       // Parse output and calculate time until expiration
       // Return time remaining
   }
   ```

2. **Renew before expiration**
   - Renew at 50% of ticket lifetime
   - Handle renewal failures gracefully
   - Log renewal events

3. **Validate configuration on startup**
   ```go
   func validateKerberosSetup() error {
       // Check if krb5.conf exists
       if _, err := os.Stat("/etc/krb5.conf"); err != nil {
           return fmt.Errorf("krb5.conf not found: %w", err)
       }

       // Verify ticket cache exists
       cmd := exec.Command("klist", "-s")
       if err := cmd.Run(); err != nil {
           return fmt.Errorf("no valid Kerberos ticket: %w", err)
       }

       return nil
   }
   ```

### Development

1. **Test with both Kerberos and password auth**
2. **Document Kerberos requirements** for users
3. **Provide fallback to password auth** when Kerberos fails
4. **Log authentication method used** for debugging

## Environment Variables

```bash
# Custom Kerberos config file
export KRB5_CONFIG=/path/to/krb5.conf

# Custom ticket cache location
export KRB5CCNAME=/tmp/krb5cc_myapp

# Enable Kerberos debug logging
export KRB5_TRACE=/dev/stderr
```

## References

- [MIT Kerberos Documentation](https://web.mit.edu/kerberos/krb5-latest/doc/)
- [Active Directory Kerberos](https://docs.microsoft.com/en-us/windows-server/security/kerberos/)
- [go-smb2 Kerberos Support](https://github.com/hirochachacha/go-smb2)
- [RFC 4120 - Kerberos V5](https://tools.ietf.org/html/rfc4120)

## Support

For Kerberos-specific issues:
1. Check this guide's troubleshooting section
2. Verify Kerberos setup with `kinit` and `klist`
3. Test with `smbclient` to isolate smbfs vs. Kerberos issues
4. Review go-smb2 library documentation
5. Open an issue with full error messages and configuration (sanitized)
