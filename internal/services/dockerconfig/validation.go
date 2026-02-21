// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024-2026 usulnet contributors
// https://github.com/fr4nsys/usulnet

package dockerconfig

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"
)

// ValidateConfig validates the full DaemonConfig and returns all errors.
func ValidateConfig(cfg *DaemonConfig) []ValidationError {
	var errs []ValidationError

	// Network validations
	if cfg.BIP != nil && *cfg.BIP != "" {
		if err := validateCIDR(*cfg.BIP); err != nil {
			errs = append(errs, ValidationError{Field: "bip", Message: err.Error()})
		}
	}
	if cfg.FixedCIDR != nil && *cfg.FixedCIDR != "" {
		if err := validateCIDR(*cfg.FixedCIDR); err != nil {
			errs = append(errs, ValidationError{Field: "fixed-cidr", Message: err.Error()})
		}
	}
	if cfg.DefaultGateway != nil && *cfg.DefaultGateway != "" {
		if err := validateIP(*cfg.DefaultGateway); err != nil {
			errs = append(errs, ValidationError{Field: "default-gateway", Message: err.Error()})
		}
	}
	for _, d := range cfg.DNS {
		if err := validateIP(d); err != nil {
			errs = append(errs, ValidationError{Field: "dns", Message: fmt.Sprintf("invalid DNS server %q: %s", d, err)})
		}
	}
	if cfg.MTU != nil && (*cfg.MTU < 68 || *cfg.MTU > 9000) {
		errs = append(errs, ValidationError{Field: "mtu", Message: "MTU must be between 68 and 9000"})
	}
	for _, pool := range cfg.DefaultAddressPools {
		if err := validateCIDR(pool.Base); err != nil {
			errs = append(errs, ValidationError{Field: "default-address-pools", Message: fmt.Sprintf("invalid base CIDR %q: %s", pool.Base, err)})
		}
		if pool.Size < 1 || pool.Size > 32 {
			errs = append(errs, ValidationError{Field: "default-address-pools", Message: fmt.Sprintf("subnet size must be 1-32, got %d", pool.Size)})
		}
	}

	// Logging validations
	if cfg.LogDriver != nil && *cfg.LogDriver != "" {
		validDrivers := map[string]bool{
			"json-file": true, "syslog": true, "journald": true,
			"gelf": true, "fluentd": true, "awslogs": true,
			"splunk": true, "gcplogs": true, "local": true, "none": true,
		}
		if !validDrivers[*cfg.LogDriver] {
			errs = append(errs, ValidationError{Field: "log-driver", Message: fmt.Sprintf("invalid log driver %q", *cfg.LogDriver)})
		}
	}
	if cfg.LogLevel != nil && *cfg.LogLevel != "" {
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true, "fatal": true}
		if !validLevels[*cfg.LogLevel] {
			errs = append(errs, ValidationError{Field: "log-level", Message: "must be one of: debug, info, warn, error, fatal"})
		}
	}
	if cfg.LogFormat != nil && *cfg.LogFormat != "" {
		if *cfg.LogFormat != "text" && *cfg.LogFormat != "json" {
			errs = append(errs, ValidationError{Field: "log-format", Message: "must be 'text' or 'json'"})
		}
	}

	// Registry validations
	for _, mirror := range cfg.RegistryMirrors {
		if !strings.HasPrefix(mirror, "http://") && !strings.HasPrefix(mirror, "https://") {
			errs = append(errs, ValidationError{Field: "registry-mirrors", Message: fmt.Sprintf("mirror %q must start with http:// or https://", mirror)})
		}
	}

	// Runtime validations
	if cfg.StorageDriver != nil && *cfg.StorageDriver != "" {
		validStorageDrivers := map[string]bool{
			"overlay2": true, "fuse-overlayfs": true, "btrfs": true,
			"zfs": true, "vfs": true,
		}
		if !validStorageDrivers[*cfg.StorageDriver] {
			errs = append(errs, ValidationError{Field: "storage-driver", Message: fmt.Sprintf("invalid storage driver %q", *cfg.StorageDriver)})
		}
	}
	if cfg.DataRoot != nil && *cfg.DataRoot != "" {
		if err := validateAbsPath(*cfg.DataRoot); err != nil {
			errs = append(errs, ValidationError{Field: "data-root", Message: err.Error()})
		}
	}
	if cfg.DefaultCgroupnsMode != nil && *cfg.DefaultCgroupnsMode != "" {
		if *cfg.DefaultCgroupnsMode != "private" && *cfg.DefaultCgroupnsMode != "host" {
			errs = append(errs, ValidationError{Field: "default-cgroupns-mode", Message: "must be 'private' or 'host'"})
		}
	}

	// Security validations
	if cfg.SeccompProfile != nil && *cfg.SeccompProfile != "" {
		if *cfg.SeccompProfile != "unconfined" && *cfg.SeccompProfile != "builtin" {
			if err := validateAbsPath(*cfg.SeccompProfile); err != nil {
				errs = append(errs, ValidationError{Field: "seccomp-profile", Message: err.Error()})
			}
		}
	}

	// Proxy validations
	if cfg.Proxies != nil {
		if cfg.Proxies.HTTPProxy != nil && *cfg.Proxies.HTTPProxy != "" {
			if !strings.HasPrefix(*cfg.Proxies.HTTPProxy, "http://") && !strings.HasPrefix(*cfg.Proxies.HTTPProxy, "https://") && !strings.HasPrefix(*cfg.Proxies.HTTPProxy, "socks5://") {
				errs = append(errs, ValidationError{Field: "proxies.http-proxy", Message: "must start with http://, https://, or socks5://"})
			}
		}
		if cfg.Proxies.HTTPSProxy != nil && *cfg.Proxies.HTTPSProxy != "" {
			if !strings.HasPrefix(*cfg.Proxies.HTTPSProxy, "http://") && !strings.HasPrefix(*cfg.Proxies.HTTPSProxy, "https://") && !strings.HasPrefix(*cfg.Proxies.HTTPSProxy, "socks5://") {
				errs = append(errs, ValidationError{Field: "proxies.https-proxy", Message: "must start with http://, https://, or socks5://"})
			}
		}
	}

	// Ulimits validations
	for name, ul := range cfg.DefaultUlimits {
		if ul.Soft > ul.Hard {
			errs = append(errs, ValidationError{Field: "default-ulimits", Message: fmt.Sprintf("ulimit %q: soft (%d) cannot exceed hard (%d)", name, ul.Soft, ul.Hard)})
		}
		if ul.Soft < 0 || ul.Hard < 0 {
			errs = append(errs, ValidationError{Field: "default-ulimits", Message: fmt.Sprintf("ulimit %q: values cannot be negative", name)})
		}
	}

	// Shutdown timeout
	if cfg.ShutdownTimeout != nil && *cfg.ShutdownTimeout < 0 {
		errs = append(errs, ValidationError{Field: "shutdown-timeout", Message: "cannot be negative"})
	}

	// Concurrent limits
	if cfg.MaxConcurrentDownloads != nil && *cfg.MaxConcurrentDownloads < 1 {
		errs = append(errs, ValidationError{Field: "max-concurrent-downloads", Message: "must be at least 1"})
	}
	if cfg.MaxConcurrentUploads != nil && *cfg.MaxConcurrentUploads < 1 {
		errs = append(errs, ValidationError{Field: "max-concurrent-uploads", Message: "must be at least 1"})
	}
	if cfg.MaxDownloadAttempts != nil && *cfg.MaxDownloadAttempts < 1 {
		errs = append(errs, ValidationError{Field: "max-download-attempts", Message: "must be at least 1"})
	}

	return errs
}

func validateCIDR(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR notation: %s", cidr)
	}
	return nil
}

func validateIP(ip string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}
	return nil
}

func validateAbsPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute: %s", path)
	}
	return nil
}

// FormatValidationErrors joins validation errors into a single message.
func FormatValidationErrors(errs []ValidationError) string {
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return strings.Join(msgs, "; ")
}
