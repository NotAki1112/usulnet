// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024-2026 usulnet contributors
// https://github.com/fr4nsys/usulnet

package web

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fr4nsys/usulnet/internal/services/dockerconfig"
	dctmpl "github.com/fr4nsys/usulnet/internal/web/templates/pages/dockerconfig"
)

// DockerConfigTempl renders the Docker daemon configuration page.
func (h *Handler) DockerConfigTempl(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pageData := h.prepareTemplPageData(r, "Docker Daemon Config", "docker-config")

	if h.dockerConfigSvc == nil {
		data := dctmpl.DockerConfigData{
			PageData:       pageData,
			WarningMessage: "Docker daemon configuration service is not available.",
		}
		h.renderTempl(w, r, dctmpl.DockerConfig(data))
		return
	}

	// Read current daemon.json
	cfg, err := h.dockerConfigSvc.Read(ctx)
	if err != nil {
		data := dctmpl.DockerConfigData{
			PageData:       pageData,
			WarningMessage: fmt.Sprintf("Failed to read daemon.json: %s", err),
		}
		h.renderTempl(w, r, dctmpl.DockerConfig(data))
		return
	}

	// Read raw JSON
	rawJSON, _ := h.dockerConfigSvc.ReadRaw(ctx)

	// Read Docker daemon info from API
	var daemonInfo dctmpl.DaemonInfoView
	containerSvc := h.services.Containers()
	if containerSvc != nil {
		if client, err := containerSvc.GetDockerClient(ctx); err == nil {
			if info, err := client.Info(ctx); err == nil {
				daemonInfo = dctmpl.DaemonInfoView{
					ServerVersion:  info.ServerVersion,
					StorageDriver:  info.StorageDriver,
					LoggingDriver:  info.LoggingDriver,
					DefaultRuntime: info.DefaultRuntime,
					DockerRootDir:  info.DockerRootDir,
					OS:             info.OS,
					Architecture:   info.Architecture,
					NCPU:           info.NCPU,
					MemTotal:       formatBytes(info.MemTotal),
					SecurityOpts:   info.SecurityOptions,
					Runtimes:       info.Runtimes,
				}
			}
		}
	}

	// List backups
	backups, _ := h.dockerConfigSvc.ListBackups(ctx)

	activeTab := r.URL.Query().Get("tab")
	if activeTab == "" {
		activeTab = "network"
	}

	data := dctmpl.DockerConfigData{
		PageData:   pageData,
		Config:     convertDaemonConfig(cfg),
		DaemonInfo: daemonInfo,
		Backups:    convertBackups(backups),
		ActiveTab:  activeTab,
		RawJSON:    rawJSON,
	}

	h.renderTempl(w, r, dctmpl.DockerConfig(data))
}

// DockerConfigUpdateNetwork handles the Network tab form submission.
func (h *Handler) DockerConfigUpdateNetwork(w http.ResponseWriter, r *http.Request) {
	h.dockerConfigUpdateCategory(w, r, "network", func(r *http.Request) map[string]interface{} {
		changes := map[string]interface{}{
			"bip":             r.FormValue("bip"),
			"fixed_cidr":     r.FormValue("fixed_cidr"),
			"default_gateway": r.FormValue("default_gateway"),
			"dns":            r.FormValue("dns"),
			"dns_search":     r.FormValue("dns_search"),
			"dns_opts":       r.FormValue("dns_opts"),
			"mtu":            r.FormValue("mtu"),
			"icc":            r.FormValue("icc") == "on",
			"ipv6":           r.FormValue("ipv6") == "on",
			"ip_forward":     r.FormValue("ip_forward") == "on",
			"ip_masq":        r.FormValue("ip_masq") == "on",
		}
		// Parse address pools (dynamic fields: pool_base_0, pool_size_0, ...)
		var pools []dockerconfig.AddressPool
		for i := 0; i < 10; i++ {
			base := strings.TrimSpace(r.FormValue(fmt.Sprintf("pool_base_%d", i)))
			sizeStr := strings.TrimSpace(r.FormValue(fmt.Sprintf("pool_size_%d", i)))
			if base == "" {
				continue
			}
			size, _ := strconv.Atoi(sizeStr)
			if size == 0 {
				size = 24
			}
			pools = append(pools, dockerconfig.AddressPool{Base: base, Size: size})
		}
		changes["default_address_pools"] = pools
		return changes
	})
}

// DockerConfigUpdateLogging handles the Logging tab form submission.
func (h *Handler) DockerConfigUpdateLogging(w http.ResponseWriter, r *http.Request) {
	h.dockerConfigUpdateCategory(w, r, "logging", func(r *http.Request) map[string]interface{} {
		changes := map[string]interface{}{
			"log_driver": r.FormValue("log_driver"),
			"log_level":  r.FormValue("log_level"),
			"log_format": r.FormValue("log_format"),
		}
		// Parse log options (dynamic key-value pairs)
		logOpts := make(map[string]string)
		for i := 0; i < 10; i++ {
			key := strings.TrimSpace(r.FormValue(fmt.Sprintf("log_opt_key_%d", i)))
			val := strings.TrimSpace(r.FormValue(fmt.Sprintf("log_opt_val_%d", i)))
			if key != "" {
				logOpts[key] = val
			}
		}
		changes["log_opts"] = logOpts
		return changes
	})
}

// DockerConfigUpdateRegistry handles the Registry tab form submission.
func (h *Handler) DockerConfigUpdateRegistry(w http.ResponseWriter, r *http.Request) {
	h.dockerConfigUpdateCategory(w, r, "registry", func(r *http.Request) map[string]interface{} {
		return map[string]interface{}{
			"registry_mirrors":                 r.FormValue("registry_mirrors"),
			"insecure_registries":              r.FormValue("insecure_registries"),
			"allow_nondistributable_artifacts": r.FormValue("allow_nondistributable_artifacts"),
		}
	})
}

// DockerConfigUpdateRuntime handles the Runtime tab form submission.
func (h *Handler) DockerConfigUpdateRuntime(w http.ResponseWriter, r *http.Request) {
	h.dockerConfigUpdateCategory(w, r, "runtime", func(r *http.Request) map[string]interface{} {
		changes := map[string]interface{}{
			"default_runtime":      r.FormValue("default_runtime"),
			"live_restore":         r.FormValue("live_restore") == "on",
			"userland_proxy":       r.FormValue("userland_proxy") == "on",
			"iptables":             r.FormValue("iptables") == "on",
			"ip6tables":            r.FormValue("ip6tables") == "on",
			"init":                 r.FormValue("init") == "on",
			"exec_opts":            r.FormValue("exec_opts"),
			"default_cgroupns_mode": r.FormValue("default_cgroupns_mode"),
			"storage_driver":       r.FormValue("storage_driver"),
			"storage_opts":         r.FormValue("storage_opts"),
			"data_root":            r.FormValue("data_root"),
		}
		// Parse runtimes (dynamic fields)
		runtimes := make(map[string]dockerconfig.Runtime)
		for i := 0; i < 10; i++ {
			name := strings.TrimSpace(r.FormValue(fmt.Sprintf("runtime_name_%d", i)))
			path := strings.TrimSpace(r.FormValue(fmt.Sprintf("runtime_path_%d", i)))
			if name == "" || path == "" {
				continue
			}
			runtimes[name] = dockerconfig.Runtime{Path: path}
		}
		changes["runtimes"] = runtimes
		return changes
	})
}

// DockerConfigUpdateProxy handles the Proxy tab form submission.
func (h *Handler) DockerConfigUpdateProxy(w http.ResponseWriter, r *http.Request) {
	h.dockerConfigUpdateCategory(w, r, "proxy", func(r *http.Request) map[string]interface{} {
		return map[string]interface{}{
			"http_proxy":  r.FormValue("http_proxy"),
			"https_proxy": r.FormValue("https_proxy"),
			"no_proxy":    r.FormValue("no_proxy"),
		}
	})
}

// DockerConfigUpdateSecurity handles the Security tab form submission.
func (h *Handler) DockerConfigUpdateSecurity(w http.ResponseWriter, r *http.Request) {
	h.dockerConfigUpdateCategory(w, r, "security", func(r *http.Request) map[string]interface{} {
		changes := map[string]interface{}{
			"no_new_privileges":     r.FormValue("no_new_privileges") == "on",
			"seccomp_profile":       r.FormValue("seccomp_profile"),
			"selinux_enabled":       r.FormValue("selinux_enabled") == "on",
			"userns_remap":          r.FormValue("userns_remap"),
			"authorization_plugins": r.FormValue("authorization_plugins"),
		}
		// Parse ulimits (dynamic fields)
		ulimits := make(map[string]dockerconfig.Ulimit)
		for i := 0; i < 10; i++ {
			name := strings.TrimSpace(r.FormValue(fmt.Sprintf("ulimit_name_%d", i)))
			softStr := strings.TrimSpace(r.FormValue(fmt.Sprintf("ulimit_soft_%d", i)))
			hardStr := strings.TrimSpace(r.FormValue(fmt.Sprintf("ulimit_hard_%d", i)))
			if name == "" {
				continue
			}
			soft, _ := strconv.ParseInt(softStr, 10, 64)
			hard, _ := strconv.ParseInt(hardStr, 10, 64)
			ulimits[name] = dockerconfig.Ulimit{Name: name, Soft: soft, Hard: hard}
		}
		changes["default_ulimits"] = ulimits
		return changes
	})
}

// DockerConfigUpdateGeneral handles the General settings form submission.
func (h *Handler) DockerConfigUpdateGeneral(w http.ResponseWriter, r *http.Request) {
	h.dockerConfigUpdateCategory(w, r, "general", func(r *http.Request) map[string]interface{} {
		return map[string]interface{}{
			"debug":                    r.FormValue("debug") == "on",
			"labels":                   r.FormValue("labels"),
			"shutdown_timeout":         r.FormValue("shutdown_timeout"),
			"max_concurrent_downloads": r.FormValue("max_concurrent_downloads"),
			"max_concurrent_uploads":   r.FormValue("max_concurrent_uploads"),
			"max_download_attempts":    r.FormValue("max_download_attempts"),
			"experimental":             r.FormValue("experimental") == "on",
			"metrics_addr":             r.FormValue("metrics_addr"),
		}
	})
}

// DockerConfigReload sends SIGHUP to the Docker daemon.
func (h *Handler) DockerConfigReload(w http.ResponseWriter, r *http.Request) {
	if h.dockerConfigSvc == nil {
		h.setFlash(w, r, "error", "Docker config service unavailable")
		http.Redirect(w, r, "/config/docker", http.StatusSeeOther)
		return
	}
	if err := h.dockerConfigSvc.ReloadDaemon(r.Context()); err != nil {
		h.setFlash(w, r, "error", "Failed to reload Docker daemon: "+err.Error())
	} else {
		h.setFlash(w, r, "success", "Docker daemon reloaded successfully (SIGHUP sent). Live-reloadable settings are now active.")
	}
	http.Redirect(w, r, "/config/docker", http.StatusSeeOther)
}

// DockerConfigRestart restarts the Docker daemon via systemctl.
func (h *Handler) DockerConfigRestart(w http.ResponseWriter, r *http.Request) {
	if h.dockerConfigSvc == nil {
		h.setFlash(w, r, "error", "Docker config service unavailable")
		http.Redirect(w, r, "/config/docker", http.StatusSeeOther)
		return
	}
	if err := h.dockerConfigSvc.RestartDaemon(r.Context()); err != nil {
		h.setFlash(w, r, "error", "Failed to restart Docker daemon: "+err.Error())
	} else {
		h.setFlash(w, r, "success", "Docker daemon restarted successfully. All settings are now active.")
	}
	http.Redirect(w, r, "/config/docker", http.StatusSeeOther)
}

// DockerConfigRestoreBackup restores a daemon.json backup.
func (h *Handler) DockerConfigRestoreBackup(w http.ResponseWriter, r *http.Request) {
	if h.dockerConfigSvc == nil {
		h.setFlash(w, r, "error", "Docker config service unavailable")
		http.Redirect(w, r, "/config/docker?tab=backups", http.StatusSeeOther)
		return
	}
	name := chi.URLParam(r, "name")
	if name == "" {
		h.setFlash(w, r, "error", "Backup name required")
		http.Redirect(w, r, "/config/docker?tab=backups", http.StatusSeeOther)
		return
	}
	if err := h.dockerConfigSvc.RestoreBackup(r.Context(), name); err != nil {
		h.setFlash(w, r, "error", "Failed to restore backup: "+err.Error())
	} else {
		h.setFlash(w, r, "success", fmt.Sprintf("Backup %s restored. Reload or restart Docker to apply.", name))
	}
	http.Redirect(w, r, "/config/docker?tab=backups", http.StatusSeeOther)
}

// dockerConfigUpdateCategory is the shared handler for all tab form submissions.
func (h *Handler) dockerConfigUpdateCategory(w http.ResponseWriter, r *http.Request, category string, extractChanges func(*http.Request) map[string]interface{}) {
	if h.dockerConfigSvc == nil {
		h.setFlash(w, r, "error", "Docker config service unavailable")
		http.Redirect(w, r, "/config/docker", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.setFlash(w, r, "error", "Invalid form data")
		http.Redirect(w, r, "/config/docker?tab="+category, http.StatusSeeOther)
		return
	}

	changes := extractChanges(r)

	result, err := h.dockerConfigSvc.UpdateCategory(r.Context(), category, changes)
	if err != nil {
		h.setFlash(w, r, "error", fmt.Sprintf("Failed to update %s settings: %s", category, err.Error()))
		http.Redirect(w, r, "/config/docker?tab="+category, http.StatusSeeOther)
		return
	}

	backupName := ""
	if result.BackupPath != "" {
		backupName = filepath.Base(result.BackupPath)
	}

	msg := fmt.Sprintf("%s settings saved successfully.", strings.Title(category))
	if backupName != "" {
		msg += fmt.Sprintf(" Backup: %s.", backupName)
	}
	if result.ApplyMode == dockerconfig.ApplyRestart {
		msg += " Docker restart required to apply changes."
	} else {
		msg += " Use Reload to apply live-reloadable settings."
	}
	h.setFlash(w, r, "success", msg)
	http.Redirect(w, r, "/config/docker?tab="+category, http.StatusSeeOther)
}

// convertDaemonConfig converts the service DaemonConfig to the template view.
func convertDaemonConfig(cfg *dockerconfig.DaemonConfig) dctmpl.DaemonConfigView {
	v := dctmpl.DaemonConfigView{}

	// General
	v.Debug = ptrBool(cfg.Debug)
	v.Labels = strings.Join(cfg.Labels, "\n")
	v.ShutdownTimeout = ptrInt(cfg.ShutdownTimeout)
	v.MaxConcurrentDownloads = ptrInt(cfg.MaxConcurrentDownloads)
	v.MaxConcurrentUploads = ptrInt(cfg.MaxConcurrentUploads)
	v.MaxDownloadAttempts = ptrInt(cfg.MaxDownloadAttempts)
	v.Experimental = ptrBool(cfg.Experimental)
	v.MetricsAddr = ptrString(cfg.MetricsAddr)

	// Network
	v.BIP = ptrString(cfg.BIP)
	v.FixedCIDR = ptrString(cfg.FixedCIDR)
	v.DefaultGateway = ptrString(cfg.DefaultGateway)
	v.DNS = strings.Join(cfg.DNS, "\n")
	v.DNSSearch = strings.Join(cfg.DNSSearch, "\n")
	v.DNSOpts = strings.Join(cfg.DNSOpts, "\n")
	v.MTU = ptrInt(cfg.MTU)
	v.ICC = ptrBool(cfg.ICC)
	v.IPv6 = ptrBool(cfg.IPv6)
	v.IPForward = ptrBool(cfg.IPForward)
	v.IPMasq = ptrBool(cfg.IPMasq)
	for _, p := range cfg.DefaultAddressPools {
		v.DefaultAddressPools = append(v.DefaultAddressPools, dctmpl.AddressPoolView{Base: p.Base, Size: p.Size})
	}

	// Logging
	v.LogDriver = ptrString(cfg.LogDriver)
	v.LogLevel = ptrString(cfg.LogLevel)
	v.LogFormat = ptrString(cfg.LogFormat)
	for k, val := range cfg.LogOpts {
		v.LogOpts = append(v.LogOpts, dctmpl.KeyValueView{Key: k, Value: val})
	}

	// Registry
	v.RegistryMirrors = strings.Join(cfg.RegistryMirrors, "\n")
	v.InsecureRegistries = strings.Join(cfg.InsecureRegistries, "\n")
	v.AllowNondistributableArtifacts = strings.Join(cfg.AllowNondistributableArtifacts, "\n")

	// Runtime
	v.DefaultRuntime = ptrString(cfg.DefaultRuntime)
	v.LiveRestore = ptrBool(cfg.LiveRestore)
	v.UserlandProxy = ptrBool(cfg.UserlandProxy)
	v.Iptables = ptrBool(cfg.Iptables)
	v.IP6Tables = ptrBool(cfg.IP6Tables)
	v.Init = ptrBool(cfg.Init)
	v.ExecOpts = strings.Join(cfg.ExecOpts, "\n")
	v.DefaultCgroupnsMode = ptrString(cfg.DefaultCgroupnsMode)
	v.StorageDriver = ptrString(cfg.StorageDriver)
	v.StorageOpts = strings.Join(cfg.StorageOpts, "\n")
	v.DataRoot = ptrString(cfg.DataRoot)
	for name, rt := range cfg.Runtimes {
		v.Runtimes = append(v.Runtimes, dctmpl.RuntimeView{Name: name, Path: rt.Path, Args: strings.Join(rt.Args, " ")})
	}

	// Proxy
	if cfg.Proxies != nil {
		v.HTTPProxy = ptrString(cfg.Proxies.HTTPProxy)
		v.HTTPSProxy = ptrString(cfg.Proxies.HTTPSProxy)
		v.NoProxy = ptrString(cfg.Proxies.NoProxy)
	}

	// Security
	v.NoNewPrivileges = ptrBool(cfg.NoNewPrivileges)
	v.SeccompProfile = ptrString(cfg.SeccompProfile)
	v.SELinuxEnabled = ptrBool(cfg.SELinuxEnabled)
	v.UsernsRemap = ptrString(cfg.UsernsRemap)
	v.AuthorizationPlugins = strings.Join(cfg.AuthorizationPlugins, "\n")
	for name, ul := range cfg.DefaultUlimits {
		v.DefaultUlimits = append(v.DefaultUlimits, dctmpl.UlimitView{Name: name, Soft: ul.Soft, Hard: ul.Hard})
	}

	return v
}

func convertBackups(backups []dockerconfig.BackupInfo) []dctmpl.BackupView {
	var result []dctmpl.BackupView
	for _, b := range backups {
		result = append(result, dctmpl.BackupView{
			Name:      b.Name,
			Size:      formatBytes(b.Size),
			Timestamp: b.Timestamp.Format("2006-01-02 15:04:05"),
		})
	}
	return result
}

func ptrBool(b *bool) bool {
	if b != nil {
		return *b
	}
	return false
}

func ptrString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func ptrInt(i *int) int {
	if i != nil {
		return *i
	}
	return 0
}

// formatBytes is already defined in handler_detail.go, reused here.
