package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/installer/pkg/types/external"
)

func TestValidatePlatform(t *testing.T) {
	cases := []struct {
		name     string
		platform *external.Platform
		expected []string
	}{
		{
			name:     "nil platform",
			platform: nil,
		},
		{
			// The pre-existing shape of this platform: no Cluster API
			// provider, nothing to validate, no new error.
			name:     "no cluster api provider",
			platform: &external.Platform{PlatformName: "SomeCloud"},
		},
		{
			name: "complete cluster api provider",
			platform: &external.Platform{
				ClusterAPI: &external.ClusterAPIProvider{
					Name:           "somecloud",
					BinaryPath:     "/artifacts/controller",
					ComponentsPath: "/artifacts/components.yaml",
				},
			},
		},
		{
			name: "args are optional",
			platform: &external.Platform{
				ClusterAPI: &external.ClusterAPIProvider{
					Name:           "somecloud",
					BinaryPath:     "/artifacts/controller",
					ComponentsPath: "/artifacts/components",
					Args:           []string{"--extra-flag=1"},
				},
			},
		},
		{
			name: "missing name",
			platform: &external.Platform{
				ClusterAPI: &external.ClusterAPIProvider{
					BinaryPath:     "/artifacts/controller",
					ComponentsPath: "/artifacts/components.yaml",
				},
			},
			expected: []string{`^test-path\.clusterAPI\.name: Required value`},
		},
		{
			name: "missing binary path",
			platform: &external.Platform{
				ClusterAPI: &external.ClusterAPIProvider{
					Name:           "somecloud",
					ComponentsPath: "/artifacts/components.yaml",
				},
			},
			expected: []string{`^test-path\.clusterAPI\.binaryPath: Required value`},
		},
		{
			name: "missing components path",
			platform: &external.Platform{
				ClusterAPI: &external.ClusterAPIProvider{
					Name:       "somecloud",
					BinaryPath: "/artifacts/controller",
				},
			},
			expected: []string{`^test-path\.clusterAPI\.componentsPath: Required value`},
		},
		{
			// Every missing field is reported at once: the point of
			// field.ErrorList is that a user fixes one install-config rather
			// than rerunning to discover the next omission.
			name: "empty cluster api provider reports every field",
			platform: &external.Platform{
				ClusterAPI: &external.ClusterAPIProvider{},
			},
			expected: []string{
				`^test-path\.clusterAPI\.name: Required value`,
				`^test-path\.clusterAPI\.binaryPath: Required value`,
				`^test-path\.clusterAPI\.componentsPath: Required value`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidatePlatform(tc.platform, field.NewPath("test-path"))
			assert.Len(t, errs, len(tc.expected))
			for i, want := range tc.expected {
				assert.Regexp(t, want, errs[i].Error())
			}
		})
	}
}
