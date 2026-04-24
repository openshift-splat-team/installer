package validation

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/vsphere"
)

func validPlatform() *vsphere.Platform {
	return &vsphere.Platform{
		VCenters: []vsphere.VCenter{
			{
				Server:   "test-vcenter",
				Port:     443,
				Username: "test-username",
				Password: "test-password",
				Datacenters: []string{
					"test-datacenter",
				},
			},
		},
		FailureDomains: []vsphere.FailureDomain{
			{
				Name:   "test-east-1a",
				Region: "test-east",
				Zone:   "test-east-1a",
				Server: "test-vcenter",
				Topology: vsphere.Topology{
					Datacenter:     "test-datacenter",
					ComputeCluster: "/test-datacenter/host/test-cluster",
					Datastore:      "/test-datacenter/datastore/test-datastore",
					Networks:       []string{"test-portgroup"},
					ResourcePool:   "/test-datacenter/host/test-cluster/Resources/test-resourcepool",
					Folder:         "/test-datacenter/vm/test-folder",
				},
			},
			{
				Name:   "test-east-2a",
				Region: "test-east",
				Zone:   "test-east-2a",
				Server: "test-vcenter",
				Topology: vsphere.Topology{
					Datacenter:     "test-datacenter",
					ComputeCluster: "/test-datacenter/host/test-cluster",
					Datastore:      "/test-datacenter/datastore/test-datastore",
					Networks:       []string{"test-portgroup"},
					Folder:         "/test-datacenter/vm/test-folder",
				},
			},
		},
	}
}

func validHosts() []*vsphere.Host {
	return []*vsphere.Host{
		{
			Role: "bootstrap",
			NetworkDevice: &vsphere.NetworkDeviceSpec{
				IPAddrs: []string{
					"192.168.101.240/24",
				},
				Gateway: "192.168.101.1",
				Nameservers: []string{
					"192.168.101.2",
				},
			},
		},
		{
			Role:          "control-plane",
			FailureDomain: "test-east-1a",
			NetworkDevice: &vsphere.NetworkDeviceSpec{
				IPAddrs: []string{
					"192.168.101.241/24",
				},
				Gateway: "192.168.101.1",
				Nameservers: []string{
					"192.168.101.2",
				},
			},
		},
		{
			Role:          "control-plane",
			FailureDomain: "test-east-2a",
			NetworkDevice: &vsphere.NetworkDeviceSpec{
				IPAddrs: []string{
					"192.168.101.242/24",
				},
				Gateway: "192.168.101.1",
				Nameservers: []string{
					"192.168.101.2",
				},
			},
		},
		{
			Role:          "control-plane",
			FailureDomain: "test-east-1a",
			NetworkDevice: &vsphere.NetworkDeviceSpec{
				IPAddrs: []string{
					"192.168.101.243/24",
				},
				Gateway: "192.168.101.1",
				Nameservers: []string{
					"192.168.101.2",
				},
			},
		},
		{
			Role:          "compute",
			FailureDomain: "test-east-1a",
			NetworkDevice: &vsphere.NetworkDeviceSpec{
				IPAddrs: []string{
					"192.168.101.244/24",
				},
				Gateway: "192.168.101.1",
				Nameservers: []string{
					"192.168.101.2",
				},
			},
		},
		{
			Role:          "compute",
			FailureDomain: "test-east-2a",
			NetworkDevice: &vsphere.NetworkDeviceSpec{
				IPAddrs: []string{
					"192.168.101.245/24",
				},
				Gateway: "192.168.101.1",
				Nameservers: []string{
					"192.168.101.2",
				},
			},
		},
		{
			Role:          "compute",
			FailureDomain: "test-east-1a",
			NetworkDevice: &vsphere.NetworkDeviceSpec{
				IPAddrs: []string{
					"192.168.101.246/24",
				},
				Gateway: "192.168.101.1",
				Nameservers: []string{
					"192.168.101.2",
				},
			},
		},
	}
}

func validStaticIPInstallConfig() *types.InstallConfig {
	return &types.InstallConfig{
		FeatureSet: configv1.TechPreviewNoUpgrade,
		ControlPlane: &types.MachinePool{
			Name:     "master",
			Replicas: pointer.Int64(3),
		},
		Compute: []types.MachinePool{
			{
				Name:     "worker",
				Replicas: pointer.Int64(3),
			},
		},
	}
}

func TestValidatePlatform(t *testing.T) {
	cases := []struct {
		name          string
		config        *types.InstallConfig
		platform      *vsphere.Platform
		expectedError string
	}{
		{
			name: "Valid nodeNetworking",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.NodeNetworking = &configv1.VSpherePlatformNodeNetworking{
					External: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup",
						NetworkSubnetCIDR:        []string{"10.0.0.0/24"},
						ExcludeNetworkSubnetCIDR: []string{"10.8.0.0/24"},
					},
					Internal: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup",
						NetworkSubnetCIDR:        []string{"10.0.0.0/24"},
						ExcludeNetworkSubnetCIDR: []string{"10.8.0.0/24"},
					},
				}
				return p
			}(),
		},
		{
			name: "Valid IPv6 nodeNetworking",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.NodeNetworking = &configv1.VSpherePlatformNodeNetworking{
					External: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup",
						NetworkSubnetCIDR:        []string{"2002::1234:abcd:ffff:c0a8:101/64"},
						ExcludeNetworkSubnetCIDR: []string{"2008::1234:abcd:ffff:c0a8:101/64"},
					},
					Internal: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup",
						NetworkSubnetCIDR:        []string{"2002::1234:abcd:ffff:c0a8:101/64"},
						ExcludeNetworkSubnetCIDR: []string{"2008::1234:abcd:ffff:c0a8:101/64"},
					},
				}
				return p
			}(),
		},
		{
			name: "Invalid nodeNetworking -- invalid NetworkSubnetCIDR",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.NodeNetworking = &configv1.VSpherePlatformNodeNetworking{
					External: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup",
						NetworkSubnetCIDR:        []string{"10..0.0/24"},
						ExcludeNetworkSubnetCIDR: []string{"10.8.0.0/24"},
					},
					Internal: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup",
						NetworkSubnetCIDR:        []string{"10..0.0/24"},
						ExcludeNetworkSubnetCIDR: []string{"10.8.0.0/24"},
					},
				}
				return p
			}(),
			expectedError: `test-path.nodeNetworking.internal.networkSubnetCidr: Invalid value: "10..0.0/24": invalid CIDR address: 10..0.0/24, test-path.nodeNetworking.external.networkSubnetCidr: Invalid value: "10..0.0/24": invalid CIDR address: 10..0.0/24`,
		},
		{
			name: "Invalid nodeNetworking -- invalid IPv6 NetworkSubnetCIDR",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.NodeNetworking = &configv1.VSpherePlatformNodeNetworking{
					External: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup",
						NetworkSubnetCIDR:        []string{"2T02::1234:abcd:ffff:c0a8:101/64"},
						ExcludeNetworkSubnetCIDR: []string{"2008::1234:abcd:ffff:c0a8:101/64"},
					},
					Internal: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup",
						NetworkSubnetCIDR:        []string{"2T02::1234:abcd:ffff:c0a8:101/64"},
						ExcludeNetworkSubnetCIDR: []string{"2008::1234:abcd:ffff:c0a8:101/64"},
					},
				}
				return p
			}(),
			expectedError: `test-path.nodeNetworking.internal.networkSubnetCidr: Invalid value: "2T02::1234:abcd:ffff:c0a8:101/64": invalid CIDR address: 2T02::1234:abcd:ffff:c0a8:101/64, test-path.nodeNetworking.external.networkSubnetCidr: Invalid value: "2T02::1234:abcd:ffff:c0a8:101/64": invalid CIDR address: 2T02::1234:abcd:ffff:c0a8:101/64`,
		},
		{
			name: "Invalid nodeNetworking -- invalid ExcludeNetworkSubnetCIDR",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.NodeNetworking = &configv1.VSpherePlatformNodeNetworking{
					External: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup",
						NetworkSubnetCIDR:        []string{"10.0.0.0/24"},
						ExcludeNetworkSubnetCIDR: []string{"10..0.0/24"},
					},
					Internal: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup",
						NetworkSubnetCIDR:        []string{"10.0.0.0/24"},
						ExcludeNetworkSubnetCIDR: []string{"10..0.0/24"},
					},
				}
				return p
			}(),
			expectedError: `test-path.nodeNetworking.internal.excludeNetworkSubnetCidr: Invalid value: "10..0.0/24": invalid CIDR address: 10..0.0/24, test-path.nodeNetworking.external.excludeNetworkSubnetCidr: Invalid value: "10..0.0/24": invalid CIDR address: 10..0.0/24`,
		},
		{
			name: "Invalid nodeNetworking -- invalid IPv6 ExcludeNetworkSubnetCIDR",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.NodeNetworking = &configv1.VSpherePlatformNodeNetworking{
					External: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup",
						NetworkSubnetCIDR:        []string{"2002::1234:abcd:ffff:c0a8:101/64"},
						ExcludeNetworkSubnetCIDR: []string{"2T08::1234:abcd:ffff:c0a8:101/64"},
					},
					Internal: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup",
						NetworkSubnetCIDR:        []string{"2002::1234:abcd:ffff:c0a8:101/64"},
						ExcludeNetworkSubnetCIDR: []string{"2T08::1234:abcd:ffff:c0a8:101/64"},
					},
				}

				return p
			}(),
			expectedError: `test-path.nodeNetworking.internal.excludeNetworkSubnetCidr: Invalid value: "2T08::1234:abcd:ffff:c0a8:101/64": invalid CIDR address: 2T08::1234:abcd:ffff:c0a8:101/64, test-path.nodeNetworking.external.excludeNetworkSubnetCidr: Invalid value: "2T08::1234:abcd:ffff:c0a8:101/64": invalid CIDR address: 2T08::1234:abcd:ffff:c0a8:101/64`,
		},
		{
			name: "Invalid nodeNetworking -- invalid network name",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.NodeNetworking = &configv1.VSpherePlatformNodeNetworking{
					External: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup-99",
						NetworkSubnetCIDR:        []string{"10.0.0.0/24"},
						ExcludeNetworkSubnetCIDR: []string{"10.8.0.0/24"},
					},
					Internal: configv1.VSpherePlatformNodeNetworkingSpec{
						Network:                  "test-portgroup-99",
						NetworkSubnetCIDR:        []string{"10.0.0.0/24"},
						ExcludeNetworkSubnetCIDR: []string{"10.8.0.0/24"},
					},
				}
				return p
			}(),
			expectedError: `test-path.nodeNetworking.internal.network: Invalid value: "test-portgroup-99": network must be defined in topology, test-path.nodeNetworking.external.network: Invalid value: "test-portgroup-99": network must be defined in topology`,
		},

		{
			name: "Valid diskType",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.DiskType = "eagerZeroedThick"
				return p
			}(),
		},
		{
			name: "Invalid diskType",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.DiskType = "invalidDiskType"
				return p
			}(),
			expectedError: `^test-path\.diskType: Invalid value: "invalidDiskType": diskType must be one of \[eagerZeroedThick thick thin\]$`,
		},
		{
			name: "Additional tag IDs provided",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains[0].Topology.TagIDs = []string{
					"urn:vmomi:InventoryServiceTag:5736bf56-49f5-4667-b38c-b97e09dc9578:GLOBAL",
					"urn:vmomi:InventoryServiceTag:5736bf56-49f5-4667-b38c-b97e09dc9579:GLOBAL",
				}
				return p
			}(),
		},
		{
			name: "Datacenter as a child of a folder",
			platform: func() *vsphere.Platform {
				p := validPlatform()

				for i, v := range p.VCenters {
					for j, dc := range v.Datacenters {
						p.VCenters[i].Datacenters[j] = path.Join("/dcfolder", dc)
					}
				}

				for i, fd := range p.FailureDomains {
					dcAsChild := path.Join("/dcfolder", fd.Topology.Datacenter)

					p.FailureDomains[i].Topology.Datacenter = dcAsChild
					p.FailureDomains[i].Topology.ResourcePool = strings.ReplaceAll(fd.Topology.ResourcePool, fd.Topology.Datacenter, dcAsChild)
					p.FailureDomains[i].Topology.Folder = strings.ReplaceAll(fd.Topology.Folder, fd.Topology.Datacenter, dcAsChild)
					p.FailureDomains[i].Topology.ComputeCluster = strings.ReplaceAll(fd.Topology.ComputeCluster, fd.Topology.Datacenter, dcAsChild)
					p.FailureDomains[i].Topology.Datastore = strings.ReplaceAll(fd.Topology.Datastore, fd.Topology.Datacenter, dcAsChild)
				}

				return p
			}(),
		},
		{
			name: "Additional invalid tag IDs provided",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains[0].Topology.TagIDs = []string{
					"urn:bad:InventoryServiceTag:5736bf56-49f5-4667-b38c-b97e09dc9578:GLOBAL",
					"urn:bad:InventoryServiceTag:5736bf56-49f5-4667-b38c-b97e09dc9579:GLOBAL",
				}
				return p
			}(),
			expectedError: `^test-path\.failureDomains\.topology\.tagIDs\: Invalid value\:.*?: tag ID must be in the format of urn\:vmomi\:InventoryServiceTag\:<UUID>\:GLOBAL$`,
		},

		{
			name:     "Valid Multi-zone platform",
			platform: validPlatform(),
		},
		{
			name: "Multi-zone platform duplicated zone names",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains[1].Zone = "test-east-1a"
				return p
			}(),
			expectedError: `^test-path.failureDomains.zone: Invalid value: "test-east-1a": cannot be used more than once for the failure domain region "test-east"`,
		},
		{
			name: "Multi-zone platform missing failureDomains",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains = make([]vsphere.FailureDomain, 0)
				return p
			}(),
			expectedError: `^test-path.failureDomains: Required value: must be defined`,
		},
		{
			name: "Multi-zone platform vCenter missing server",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.VCenters[0].Server = ""
				return p
			}(),
			expectedError: `test-path\.vcenters\[0]\.server: Required value: must be the domain name or IP address of the vCenter(.*)`,
		},
		{
			name: "Multi-zone platform Capital letters in vCenter",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.VCenters[0].Server = "tEsT-vCenter"
				return p
			}(),
			expectedError: `(.*)test-path\.vcenters\[0].server: Invalid value: "tEsT-vCenter": must be the domain name or IP address of the vCenter`,
		},
		{
			name: "Multi-zone missing username",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.VCenters[0].Username = ""
				return p
			}(),
			expectedError: `^test-path\.vcenters\[0].username: Required value: must specify the username$`,
		},
		{
			name: "Multi-zone missing password",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.VCenters[0].Password = ""
				return p
			}(),
			expectedError: `^test-path\.vcenters\[0].password: Required value: must specify the password$`,
		},
		{
			name: "Multi-zone missing datacenter",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.VCenters[0].Datacenters = []string{}
				return p
			}(),
			expectedError: `^test-path\.vcenters\[0].datacenters: Required value: must specify at least one datacenter$`,
		},
		{
			name: "Multi-zone platform wrong vCenter name in failureDomain zone",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains[0].Server = "bad-vcenter"
				return p
			}(),
			expectedError: `^test-path\.failureDomains\.server: Invalid value: "bad-vcenter": server does not exist in vcenters`,
		},
		{
			name: "Multi-zone platform failure domain topology cluster relative path",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains[0].Topology.ComputeCluster = "incomplete-path"
				p.FailureDomains[0].Topology.ResourcePool = "/test-datacenter/host/incomplete-path/Resources/test-resourcepool"
				return p
			}(),
			expectedError: `(.*)test-path\.failureDomains\.topology\.computeCluster: Invalid value: "incomplete-path": full path of compute cluster must be provided in format /<datacenter>/host/<cluster>`,
		},
		{
			name: "Multi-zone platform datacenter in failure domain topology doesn't match cluster datacenter",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains[0].Topology.ComputeCluster = "/other-datacenter/host/cluster"
				return p
			}(),
			expectedError: `^test-path.failureDomains.topology.computeCluster: Invalid value: "/other-datacenter/host/cluster": compute cluster must be in datacenter test-datacenter`,
		},
		{
			name: "Multi-zone platform failureDomain missing name",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains[0].Name = ""
				return p
			}(),
			expectedError: `^test-path\.failureDomains\.name: Required value: must specify the name`,
		},
		{
			name: "Multi-zone platform failureDomain region missing name",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains[0].Region = ""
				return p
			}(),
			expectedError: `^test-path\.failureDomains\.region: Required value: must specify region tag value`,
		},
		{
			name: "Multi-zone platform failureDomain zone missing name",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains[0].Name = ""
				return p
			}(),
			expectedError: `^test-path\.failureDomains\.name: Required value: must specify the name`,
		},
		{
			name: "Multi-zone platform failureDomain duplicate names",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains[1].Name = p.FailureDomains[0].Name
				return p
			}(),
			expectedError: `test-path\.failureDomains\.name\[1\]: Duplicate value: "test-east-1a"`,
		},
		{
			name: "Multi-zone platform failureDomain zone missing tag category",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains[0].Zone = ""
				return p
			}(),
			expectedError: `^test-path\.failureDomains\.zone: Required value: must specify zone tag value`,
		},
		{
			name:     "allowed load balancer field with OpenShift managed default",
			platform: validPlatform(),
			config: &types.InstallConfig{
				FeatureSet: configv1.TechPreviewNoUpgrade,
				Platform: types.Platform{
					VSphere: func() *vsphere.Platform {
						p := validPlatform()
						p.LoadBalancer = &configv1.VSpherePlatformLoadBalancer{
							Type: configv1.LoadBalancerTypeOpenShiftManagedDefault,
						}
						return p
					}(),
				},
			},
		},
		{
			name:     "allowed load balancer field with user-managed",
			platform: validPlatform(),
			config: &types.InstallConfig{
				FeatureSet: configv1.TechPreviewNoUpgrade,
				Platform: types.Platform{
					VSphere: func() *vsphere.Platform {
						p := validPlatform()
						p.LoadBalancer = &configv1.VSpherePlatformLoadBalancer{
							Type: configv1.LoadBalancerTypeUserManaged,
						}
						return p
					}(),
				},
			},
		},
		{
			name:     "allowed load balancer field invalid type",
			platform: validPlatform(),
			config: &types.InstallConfig{
				FeatureSet: configv1.TechPreviewNoUpgrade,
				Platform: types.Platform{
					VSphere: func() *vsphere.Platform {
						p := validPlatform()
						p.LoadBalancer = &configv1.VSpherePlatformLoadBalancer{
							Type: "FooBar",
						}
						return p
					}(),
				},
			},
			expectedError: `^test-path\.loadBalancer.type: Invalid value: "FooBar": invalid load balancer type`,
		},
		{
			name: "Static IP - valid",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				return p
			}(),
			config: validStaticIPInstallConfig(),
		},
		{
			name: "Static IP - no hosts configured",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				return p
			}(),
			config: validStaticIPInstallConfig(),
		},
		{
			name: "Static IP - invalid Role",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				p.Hosts[1].Role = "crazy-uncle"
				return p
			}(),
			config:        validStaticIPInstallConfig(),
			expectedError: `test-path.hosts.role: Unsupported value: "crazy-uncle": supported values: "bootstrap", "compute", "control-plane"`,
		},
		{
			name: "Static IP - invalid FailureDomain",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				p.Hosts[1].FailureDomain = "north-pole"
				return p
			}(),
			config:        validStaticIPInstallConfig(),
			expectedError: `^test-path.hosts.failureDomain: Invalid value: "north-pole": failure domain not found$`,
		},
		{
			name: "Static IP - missing NetworkDevice",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				p.Hosts[1].NetworkDevice = nil
				return p
			}(),
			config:        validStaticIPInstallConfig(),
			expectedError: `^test-path.hosts.networkDevice: Required value: must specify networkDevice configuration$`,
		},
		{
			name: "Static IP - missing IP",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				p.Hosts[1].NetworkDevice.IPAddrs = nil
				return p
			}(),
			config:        validStaticIPInstallConfig(),
			expectedError: `^test-path.hosts.ipAddrs: Required value: must specify a IP$`,
		},
		{
			name: "Static IP - invalid IP",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				p.Hosts[1].NetworkDevice.IPAddrs[0] = "86.7.5.309/24"
				return p
			}(),
			config:        validStaticIPInstallConfig(),
			expectedError: `^test-path.hosts.ipAddrs: Invalid value: "86.7.5.309/24": invalid CIDR address: 86.7.5.309/24$`,
		},
		{
			name: "Static IP - invalid IP blank",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				p.Hosts[1].NetworkDevice.IPAddrs[0] = ""
				return p
			}(),
			config:        validStaticIPInstallConfig(),
			expectedError: `^test-path.hosts.ipAddrs: Required value: must specify a IP address with CIDR$`,
		},
		{
			name: "Static IP - invalid IP CIDR",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				p.Hosts[1].NetworkDevice.IPAddrs[0] = "86.7.5.309/55"
				return p
			}(),
			config:        validStaticIPInstallConfig(),
			expectedError: `^test-path.hosts.ipAddrs: Invalid value: "86.7.5.309/55": invalid CIDR address: 86.7.5.309/55$`,
		},
		{
			name: "Static IP - invalid IP missing CIDR",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				p.Hosts[1].NetworkDevice.IPAddrs[0] = "86.7.5.309"
				return p
			}(),
			config:        validStaticIPInstallConfig(),
			expectedError: `^test-path.hosts.ipAddrs: Invalid value: "86.7.5.309": invalid CIDR address: 86.7.5.309$`,
		},
		{
			name: "Static IP - valid Gateway IPv4",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				p.Hosts[1].NetworkDevice.Gateway = "192.168.100.125"
				return p
			}(),
			config: validStaticIPInstallConfig(),
		},
		{
			name: "Static IP - invalid Gateway IPv4",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				p.Hosts[1].NetworkDevice.Gateway = "86.7.5.309"
				return p
			}(),
			config:        validStaticIPInstallConfig(),
			expectedError: `^test-path.hosts.gateway: Invalid value: "86.7.5.309": "86.7.5.309" is not a valid IP$`,
		},
		{
			name: "Static IP - valid Gateway IPv6",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				p.Hosts[1].NetworkDevice.Gateway = "2001:db8:3333:4444:5555:6666:7777:8888"
				return p
			}(),
			config: validStaticIPInstallConfig(),
		},
		{
			name: "Static IP - invalid Gateway IPv6",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				p.Hosts[1].NetworkDevice.Gateway = "8888:666:7777:5555:3333:0000:9999:JENNY"
				return p
			}(),
			config:        validStaticIPInstallConfig(),
			expectedError: `^test-path.hosts.gateway: Invalid value: "8888:666:7777:5555:3333:0000:9999:JENNY": "8888:666:7777:5555:3333:0000:9999:JENNY" is not a valid IP$`,
		},
		{
			name: "Static IP - More than 3 nameservers",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				p.Hosts[1].NetworkDevice.Nameservers = []string{"86.75.30.9", "86.75.30.8", "86.75.30.7", "86.75.30.6"}
				return p
			}(),
			config:        validStaticIPInstallConfig(),
			expectedError: `^test-path.hosts.nameservers: Too many: 4: must have at most 3 items$`,
		},
		{
			name: "Static IP - No bootstrap host",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				return p
			}(),
			config: &types.InstallConfig{
				FeatureSet: configv1.TechPreviewNoUpgrade,
				Platform: types.Platform{
					VSphere: func() *vsphere.Platform {
						p := validPlatform()
						p.Hosts = validHosts()[1:]
						return p
					}(),
				},
				ControlPlane: &types.MachinePool{
					Name:     "master",
					Replicas: pointer.Int64(3),
				},
				Compute: []types.MachinePool{
					{
						Name:     "worker",
						Replicas: pointer.Int64(3),
					},
				},
			},
			expectedError: `^test-path.hosts: Invalid value: "bootstrap": a single host with the bootstrap role must be defined$`,
		},
		{
			name: "Static IP - Multiple bootstrap hosts",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				return p
			}(),
			config: &types.InstallConfig{
				FeatureSet: configv1.TechPreviewNoUpgrade,
				Platform: types.Platform{
					VSphere: func() *vsphere.Platform {
						p := validPlatform()
						p.Hosts = append(validHosts(), validHosts()[0])
						return p
					}(),
				},
				ControlPlane: &types.MachinePool{
					Name:     "master",
					Replicas: pointer.Int64(3),
				},
				Compute: []types.MachinePool{
					{
						Name:     "worker",
						Replicas: pointer.Int64(3),
					},
				},
			},
			expectedError: `^test-path.hosts: Invalid value: "bootstrap": a single host with the bootstrap role must be defined$`,
		},
		{
			name: "Static IP - Not enough control-planes",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				return p
			}(),
			config: &types.InstallConfig{
				FeatureSet: configv1.TechPreviewNoUpgrade,
				Platform: types.Platform{
					VSphere: func() *vsphere.Platform {
						p := validPlatform()
						p.Hosts = validHosts()
						return p
					}(),
				},
				ControlPlane: &types.MachinePool{
					Name:     "master",
					Replicas: pointer.Int64(4),
				},
				Compute: []types.MachinePool{
					{
						Name:     "worker",
						Replicas: pointer.Int64(3),
					},
				},
			},
			expectedError: `^test-path.hosts: Invalid value: "control-plane": not enough hosts found \(3\) to support all the configured control plane replicas \(4\)$`,
		},
		{
			name: "Static IP - Too many control-planes",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				return p
			}(),
			config: &types.InstallConfig{
				FeatureSet: configv1.TechPreviewNoUpgrade,
				Platform: types.Platform{
					VSphere: func() *vsphere.Platform {
						p := validPlatform()
						p.Hosts = validHosts()
						return p
					}(),
				},
				ControlPlane: &types.MachinePool{
					Name:     "master",
					Replicas: pointer.Int64(2),
				},
				Compute: []types.MachinePool{
					{
						Name:     "worker",
						Replicas: pointer.Int64(3),
					},
				},
			},
			expectedError: `^test-path.hosts: Invalid value: "control-plane": too many hosts found \(3\) for the configured control plane replicas \(2\)$`,
		},
		{
			name: "Static IP - Not enough workers",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				return p
			}(),
			config: &types.InstallConfig{
				FeatureSet: configv1.TechPreviewNoUpgrade,
				Platform: types.Platform{
					VSphere: func() *vsphere.Platform {
						p := validPlatform()
						p.Hosts = validHosts()
						return p
					}(),
				},
				ControlPlane: &types.MachinePool{
					Name:     "master",
					Replicas: pointer.Int64(3),
				},
				Compute: []types.MachinePool{
					{
						Name:     "worker",
						Replicas: pointer.Int64(4),
					},
				},
			},
			expectedError: `^test-path.hosts: Invalid value: "compute": not enough hosts found \(3\) to support all the configured compute replicas \(4\)$`,
		},
		{
			name: "Static IP - Too many workers",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				return p
			}(),
			config: &types.InstallConfig{
				FeatureSet: configv1.TechPreviewNoUpgrade,
				Platform: types.Platform{
					VSphere: func() *vsphere.Platform {
						p := validPlatform()
						p.Hosts = validHosts()
						return p
					}(),
				},
				ControlPlane: &types.MachinePool{
					Name:     "master",
					Replicas: pointer.Int64(3),
				},
				Compute: []types.MachinePool{
					{
						Name:     "worker",
						Replicas: pointer.Int64(2),
					},
				},
			},
			expectedError: `^test-path.hosts: Invalid value: "compute": too many hosts found \(3\) for the configured compute replicas \(2\)$`,
		},
		{
			name: "Static IP - Not enough control-plane and workers",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.Hosts = validHosts()
				return p
			}(),
			config: &types.InstallConfig{
				FeatureSet: configv1.TechPreviewNoUpgrade,
				Platform: types.Platform{
					VSphere: func() *vsphere.Platform {
						p := validPlatform()
						p.Hosts = validHosts()
						return p
					}(),
				},
				ControlPlane: &types.MachinePool{
					Name:     "master",
					Replicas: pointer.Int64(4),
				},
				Compute: []types.MachinePool{
					{
						Name:     "worker",
						Replicas: pointer.Int64(4),
					},
				},
			},
			expectedError: `^\[test-path.hosts: Invalid value: "control-plane": not enough hosts found \(3\) to support all the configured control plane replicas \(4\), test-path.hosts: Invalid value: "compute": not enough hosts found \(3\) to support all the configured compute replicas \(4\)]$`,
		},
		{
			name: "Multi NIC - Too many NICs",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains[0].Topology.Networks = []string{"vlan_1", "vlan_2", "vlan_3", "vlan_4", "vlan_5", "vlan_6", "vlan_7", "vlan_8", "vlan_9", "vlan_10", "vlan_11"}
				return p
			}(),
			expectedError: `test-path.failureDomains.topology.networks: Too many: 11: must have at most 10 items`,
		},
		{
			name: "Multi NIC - Not enough NICs",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.FailureDomains[0].Topology.Networks = []string{}
				return p
			}(),
			expectedError: `test-path.failureDomains.topology.networks: Required value: must specify a network`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Build default wrapping installConfig
			if tc.config == nil {
				tc.config = installConfig().build()
				tc.config.VSphere = tc.platform
			}
			if tc.config.VSphere == nil {
				fmt.Printf("Setting vsphere: %v", tc.platform)
				tc.config.VSphere = tc.platform
			}
			err := ValidatePlatform(tc.config.VSphere, false, field.NewPath("test-path"), tc.config).ToAggregate()
			if tc.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Regexp(t, regexp.MustCompile(tc.expectedError), err)
			}
		})
	}
}

type installConfigBuilder struct {
	types.InstallConfig
}

func installConfig() *installConfigBuilder {
	return &installConfigBuilder{
		InstallConfig: types.InstallConfig{},
	}
}

func (icb *installConfigBuilder) build() *types.InstallConfig {
	return &icb.InstallConfig
}

// TestValidateComponentCredentials tests validation of componentCredentials in install-config.yaml
func TestValidateComponentCredentials(t *testing.T) {
	tests := []struct {
		name    string
		creds   *vsphere.ComponentCredentials
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid component credentials - all components",
			creds: &vsphere.ComponentCredentials{
				Installer: &vsphere.ComponentCredential{
					Username: "installer@vsphere.local",
					Password: "password123",
				},
				MachineAPI: &vsphere.ComponentCredential{
					Username: "machine-api@vsphere.local",
					Password: "password456",
				},
				Storage: &vsphere.ComponentCredential{
					Username: "storage@vsphere.local",
					Password: "password789",
				},
				CloudController: &vsphere.ComponentCredential{
					Username: "cloud-controller@vsphere.local",
					Password: "passwordabc",
				},
				Diagnostics: &vsphere.ComponentCredential{
					Username: "diagnostics@vsphere.local",
					Password: "passworddef",
				},
			},
			wantErr: false,
		},
		{
			name: "valid partial credentials - runtime only",
			creds: &vsphere.ComponentCredentials{
				MachineAPI: &vsphere.ComponentCredential{
					Username: "machine-api@vsphere.local",
					Password: "password",
				},
				Storage: &vsphere.ComponentCredential{
					Username: "storage@vsphere.local",
					Password: "password",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - empty username",
			creds: &vsphere.ComponentCredentials{
				MachineAPI: &vsphere.ComponentCredential{
					Username: "", // Empty username
					Password: "password",
				},
			},
			wantErr: true,
			errMsg:  "machineAPI username cannot be empty",
		},
		{
			name: "invalid - empty password",
			creds: &vsphere.ComponentCredentials{
				MachineAPI: &vsphere.ComponentCredential{
					Username: "machine-api@vsphere.local",
					Password: "", // Empty password
				},
			},
			wantErr: true,
			errMsg:  "machineAPI password cannot be empty",
		},
		{
			name: "invalid - malformed username",
			creds: &vsphere.ComponentCredentials{
				MachineAPI: &vsphere.ComponentCredential{
					Username: "invalid username with spaces",
					Password: "password",
				},
			},
			wantErr: true,
			errMsg:  "invalid username format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: Implement validation function
			// err := ValidateComponentCredentials(tt.creds, field.NewPath("test"))
			// Validate error matches expectations
			t.Skip("Implementation pending - Story #16")
		})
	}
}

// TestValidateCredentialsFile tests validation of ~/.vsphere/credentials.yaml file
func TestValidateCredentialsFile(t *testing.T) {
	tests := []struct {
		name         string
		fileContent  string
		wantErr      bool
		errMsg       string
		expectedVCs  []string
		expectedKeys []string
	}{
		{
			name: "valid credentials file - single vCenter",
			fileContent: `vcenters:
  vcenter1.example.com:
    installer:
      username: installer@vsphere.local
      password: pass1
    machine_api:
      username: machine-api@vsphere.local
      password: pass2
`,
			wantErr:      false,
			expectedVCs:  []string{"vcenter1.example.com"},
			expectedKeys: []string{"installer", "machine_api"},
		},
		{
			name: "valid credentials file - multi vCenter",
			fileContent: `vcenters:
  vcenter1.example.com:
    installer:
      username: installer@vsphere.local
      password: pass1
    machine_api:
      username: machine-api@vsphere.local
      password: pass2
  vcenter2.example.com:
    installer:
      username: installer@vc2.local
      password: pass3
    storage:
      username: storage@vc2.local
      password: pass4
`,
			wantErr:      false,
			expectedVCs:  []string{"vcenter1.example.com", "vcenter2.example.com"},
			expectedKeys: []string{"installer", "machine_api", "storage"},
		},
		{
			name: "invalid YAML format",
			fileContent: `vcenters:
  vcenter1.example.com
    - invalid: yaml
`,
			wantErr: true,
			errMsg:  "invalid YAML format",
		},
		{
			name: "missing vcenters key",
			fileContent: `other_field:
  value: test
`,
			wantErr: true,
			errMsg:  "vcenters key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: Implement credentials file parsing and validation
			// Parse YAML file content
			// Validate structure
			// Verify vCenters and component keys
			t.Skip("Implementation pending - Story #16")
		})
	}
}

// TestValidateMultiVCenterCredentials tests multi-vCenter credential validation
func TestValidateMultiVCenterCredentials(t *testing.T) {
	tests := []struct {
		name     string
		platform *vsphere.Platform
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid - credentials for all vCenters",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.VCenters = []vsphere.VCenter{
					{Server: "vcenter1.example.com", Datacenters: []string{"DC1"}},
					{Server: "vcenter2.example.com", Datacenters: []string{"DC2"}},
				}
				// ComponentCredentials covering all vCenters would be added here
				return p
			}(),
			wantErr: false,
		},
		{
			name: "invalid - missing credentials for vcenter2",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				p.VCenters = []vsphere.VCenter{
					{Server: "vcenter1.example.com", Datacenters: []string{"DC1"}},
					{Server: "vcenter2.example.com", Datacenters: []string{"DC2"}},
				}
				// Missing vcenter2 credentials
				return p
			}(),
			wantErr: true,
			errMsg:  "credentials missing for vCenter: vcenter2.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: Implement multi-vCenter validation
			// Ensure credentials exist for each configured vCenter
			t.Skip("Implementation pending - Story #16")
		})
	}
}

// TestValidateComponentNames tests that only valid component names are accepted
func TestValidateComponentNames(t *testing.T) {
	validComponents := []string{"installer", "machineAPI", "storage", "cloudController", "diagnostics"}
	invalidComponents := []string{"unknown", "custom", "foo"}

	t.Run("valid component names", func(t *testing.T) {
		for _, comp := range validComponents {
			t.Run(comp, func(t *testing.T) {
				// TODO: Validate component name is in allowed list
				t.Skip("Implementation pending - Story #16")
			})
		}
	})

	t.Run("invalid component names", func(t *testing.T) {
		for _, comp := range invalidComponents {
			t.Run(comp, func(t *testing.T) {
				// TODO: Validate component name is rejected
				t.Skip("Implementation pending - Story #16")
			})
		}
	})
}
