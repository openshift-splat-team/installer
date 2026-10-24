package external

import (
	"github.com/openshift/installer/pkg/destroy/providers"
	externaltypes "github.com/openshift/installer/pkg/types/external"
)

func init() {
	providers.Registry[externaltypes.Name] = New
}
