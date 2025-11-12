package validation

import (
	"fmt"
	"net"
	"net/url"

	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/proxmox"
	"github.com/openshift/installer/pkg/validate"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidatePlatform checks that the specified platform is valid.
func ValidatePlatform(p *proxmox.Platform, agentBasedInstallation bool, fldPath *field.Path, c *types.InstallConfig) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate Proxmox servers configuration
	if len(p.Proxmoxs) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("proxmoxs"), "must specify at least one Proxmox server"))
	} else {
		allErrs = append(allErrs, validateProxmoxServers(p.Proxmoxs, fldPath.Child("proxmoxs"))...)
	}

	// Validate API VIPs
	if len(p.APIVIPs) > 0 {
		allErrs = append(allErrs, validateVIPs(p.APIVIPs, fldPath.Child("apiVIPs"))...)
	}

	// Validate Ingress VIPs
	if len(p.IngressVIPs) > 0 {
		allErrs = append(allErrs, validateVIPs(p.IngressVIPs, fldPath.Child("ingressVIPs"))...)
	}

	return allErrs
}

// validateProxmoxServers validates the Proxmox server configurations.
func validateProxmoxServers(proxmoxs []proxmox.Proxmox, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, px := range proxmoxs {
		idxPath := fldPath.Index(i)

		// Validate URL
		if px.Url == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("url"), "must specify the Proxmox API URL"))
		} else {
			// Parse and validate the URL
			parsedURL, err := url.Parse(px.Url)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("url"), px.Url, fmt.Sprintf("invalid URL format: %v", err)))
			} else {
				// Validate the hostname/IP in the URL
				if parsedURL.Host != "" {
					host := parsedURL.Hostname()
					if err := validate.Host(host); err != nil {
						allErrs = append(allErrs, field.Invalid(idxPath.Child("url"), px.Url, fmt.Sprintf("invalid host in URL: %v", err)))
					}
				}
			}
		}

		// Validate port if specified
		if px.Port != 0 {
			if px.Port < 1 || px.Port > 65535 {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("port"), px.Port, "port must be between 1 and 65535"))
			}
		}

		// Validate token and secret
		if px.Token == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("token"), "must specify the Proxmox API token"))
		}

		if px.Secret == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("secret"), "must specify the Proxmox API secret"))
		}
	}

	return allErrs
}

// validateVIPs validates a list of VIP addresses.
func validateVIPs(vips []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Check maximum of 2 VIPs (for dual-stack)
	if len(vips) > 2 {
		allErrs = append(allErrs, field.TooMany(fldPath, len(vips), 2))
	}

	// Track IP versions to ensure we don't have duplicates
	ipv4Count := 0
	ipv6Count := 0

	for i, vip := range vips {
		ip := net.ParseIP(vip)
		if ip == nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i), vip, "must be a valid IP address"))
			continue
		}

		// Count IPv4 and IPv6 addresses
		if ip.To4() != nil {
			ipv4Count++
		} else {
			ipv6Count++
		}
	}

	// For dual-stack, we should have one IPv4 and one IPv6
	if len(vips) == 2 {
		if ipv4Count != 1 || ipv6Count != 1 {
			allErrs = append(allErrs, field.Invalid(fldPath, vips, "for dual-stack, must specify exactly one IPv4 and one IPv6 address"))
		}
	} else if len(vips) == 1 {
		// For single-stack, ensure we have either IPv4 or IPv6
		if ipv4Count != 1 && ipv6Count != 1 {
			allErrs = append(allErrs, field.Invalid(fldPath, vips, "must specify either one IPv4 or one IPv6 address"))
		}
	}

	return allErrs
}
