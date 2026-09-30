package clusterapi

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck //CORS-3563
	"sigs.k8s.io/yaml"

	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/asset/installconfig"
	"github.com/openshift/installer/pkg/asset/manifests"
	"github.com/openshift/installer/pkg/asset/manifests/aws"
	"github.com/openshift/installer/pkg/asset/manifests/azure"
	"github.com/openshift/installer/pkg/asset/manifests/capiutils"
	"github.com/openshift/installer/pkg/asset/manifests/gcp"
	"github.com/openshift/installer/pkg/asset/manifests/ibmcloud"
	"github.com/openshift/installer/pkg/asset/manifests/nutanix"
	"github.com/openshift/installer/pkg/asset/manifests/openstack"
	"github.com/openshift/installer/pkg/asset/manifests/powervs"
	"github.com/openshift/installer/pkg/asset/manifests/vsphere"
	"github.com/openshift/installer/pkg/asset/openshiftinstall"
	"github.com/openshift/installer/pkg/asset/rhcos"
	"github.com/openshift/installer/pkg/clusterapi"
	awstypes "github.com/openshift/installer/pkg/types/aws"
	azuretypes "github.com/openshift/installer/pkg/types/azure"
	baremetaltypes "github.com/openshift/installer/pkg/types/baremetal"
	externaltypes "github.com/openshift/installer/pkg/types/external"
	gcptypes "github.com/openshift/installer/pkg/types/gcp"
	ibmcloudtypes "github.com/openshift/installer/pkg/types/ibmcloud"
	nonetypes "github.com/openshift/installer/pkg/types/none"
	nutanixtypes "github.com/openshift/installer/pkg/types/nutanix"
	openstacktypes "github.com/openshift/installer/pkg/types/openstack"
	powervctypes "github.com/openshift/installer/pkg/types/powervc"
	powervstypes "github.com/openshift/installer/pkg/types/powervs"
	vsphereplatform "github.com/openshift/installer/pkg/types/vsphere"
)

var _ asset.WritableRuntimeAsset = (*Cluster)(nil)

// Cluster generates manifests for target cluster
// creation using CAPI.
type Cluster struct {
	FileList []*asset.RuntimeFile
}

// Name returns a human friendly name for the operator.
func (c *Cluster) Name() string {
	return "Cluster API Manifests"
}

// Dependencies returns all of the dependencies directly needed by the
// ClusterAPI asset.
func (c *Cluster) Dependencies() []asset.Asset {
	return []asset.Asset{
		&installconfig.InstallConfig{},
		&installconfig.ClusterID{},
		&openshiftinstall.Config{},
		&manifests.FeatureGate{},
		new(rhcos.Image),
	}
}

// Generate generates the respective operator config.yml files.
func (c *Cluster) Generate(_ context.Context, dependencies asset.Parents) error {
	installConfig := &installconfig.InstallConfig{}
	clusterID := &installconfig.ClusterID{}
	openshiftInstall := &openshiftinstall.Config{}
	featureGate := &manifests.FeatureGate{}
	rhcosImage := new(rhcos.Image)
	dependencies.Get(installConfig, clusterID, openshiftInstall, featureGate, rhcosImage)

	// If the feature gate is not enabled, do not generate any manifests.
	if !capiutils.IsEnabled(installConfig) {
		return nil
	}

	c.FileList = []*asset.RuntimeFile{}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: capiutils.Namespace,
		},
	}
	namespace.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))
	c.FileList = append(c.FileList, &asset.RuntimeFile{Object: namespace, File: asset.File{Filename: "000_capi-namespace.yaml"}})

	var out *capiutils.GenerateClusterAssetsOutput
	switch platform := installConfig.Config.Platform.Name(); platform {
	case awstypes.Name:
		var err error
		out, err = aws.GenerateClusterAssets(installConfig, clusterID)
		if err != nil {
			return errors.Wrap(err, "failed to generate AWS manifests")
		}
	case azuretypes.Name:
		var err error
		out, err = azure.GenerateClusterAssets(installConfig, clusterID)
		if err != nil {
			return errors.Wrap(err, "failed to generate Azure manifests")
		}
	case gcptypes.Name:
		var err error
		out, err = gcp.GenerateClusterAssets(installConfig, clusterID)
		if err != nil {
			return fmt.Errorf("failed to generate GCP manifests: %w", err)
		}
	case vsphereplatform.Name:
		var err error
		out, err = vsphere.GenerateClusterAssets(installConfig, clusterID)
		if err != nil {
			return fmt.Errorf("failed to generate vSphere manifests %w", err)
		}
	case openstacktypes.Name, powervctypes.Name:
		var err error
		out, err = openstack.GenerateClusterAssets(installConfig, clusterID)
		if err != nil {
			return errors.Wrap(err, "failed to generate OpenStack manifests")
		}
	case powervstypes.Name:
		var err error
		osImage := strings.SplitN(rhcosImage.ControlPlane, "/", 2)
		out, err = powervs.GenerateClusterAssets(installConfig, clusterID, osImage[0], osImage[1])
		if err != nil {
			return fmt.Errorf("failed to generate PowerVS manifests %w", err)
		}
	case nutanixtypes.Name:
		var err error
		out, err = nutanix.GenerateClusterAssets(installConfig, clusterID)
		if err != nil {
			return errors.Wrap(err, "failed to generate Nutanix manifests")
		}
	case ibmcloudtypes.Name:
		var err error
		// Isolate the RHCOS Image filename.
		imageName := strings.SplitN(filepath.Base(rhcosImage.ControlPlane), ".gz", 2)[0]
		out, err = ibmcloud.GenerateClusterAssets(installConfig, clusterID, imageName)
		if err != nil {
			return fmt.Errorf("failed to generate IBM Cloud VPC manifests: %w", err)
		}
	case externaltypes.Name:
		// The installer generates no Cluster and no infrastructure object for
		// this platform -- the user supplies them, and Load reads them. Only
		// the namespace is generated, and unlike the none and baremetal cases
		// below it has to be marshalled and given its manifest-dir path,
		// because for External these files are really applied.
		return c.finalize()
	case nonetypes.Name, baremetaltypes.Name:
		return nil
	default:
		return fmt.Errorf("unsupported platform %q", platform)
	}

	if len(out.InfrastructureRefs) == 0 {
		return fmt.Errorf("failed to generate manifests: cluster.Spec.InfrastructureRef was never set")
	}

	logrus.Infof("Adding clusters...")
	for index, infra := range out.InfrastructureRefs {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      infra.Name,
				Namespace: capiutils.Namespace,
			},
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: &clusterv1.ClusterNetwork{
					APIServerPort: ptr.To[int32](6443),
				},
			},
		}
		cluster.Spec.InfrastructureRef = infra
		cluster.SetGroupVersionKind(clusterv1.GroupVersion.WithKind("Cluster"))
		c.FileList = append(c.FileList, &asset.RuntimeFile{Object: cluster, File: asset.File{Filename: fmt.Sprintf("01_capi-cluster-%d.yaml", index)}})
	}

	// Append the infrastructure manifests.
	c.FileList = append(c.FileList, out.Manifests...)

	return c.finalize()
}

// finalize marshals every generated object and moves it under the manifest
// directory.
func (c *Cluster) finalize() error {
	for _, m := range c.FileList {
		objData, err := yaml.Marshal(m.Object)
		if err != nil {
			return errors.Wrapf(err, "failed to marshal infrastructure manifest %s", m.Filename)
		}
		m.Data = objData

		// If the filename is already a path, do not append the manifest dir.
		if filepath.Dir(m.Filename) == capiutils.ManifestDir {
			continue
		}
		m.Filename = filepath.Join(capiutils.ManifestDir, m.Filename)
	}

	asset.SortManifestFiles(c.FileList)
	return nil
}

// Files returns the files generated by the asset.
func (c *Cluster) Files() []*asset.File {
	files := []*asset.File{}
	for _, f := range c.FileList {
		files = append(files, &f.File)
	}
	return files
}

// RuntimeFiles returns the files generated by the asset.
func (c *Cluster) RuntimeFiles() []*asset.RuntimeFile {
	return c.FileList
}

// Load returns the openshift asset from disk.
func (c *Cluster) Load(f asset.FileFetcher) (bool, error) {
	yamlFileList, err := f.FetchByPattern(filepath.Join(capiutils.ManifestDir, "*.yaml"))
	if err != nil {
		return false, errors.Wrap(err, "failed to load *.yaml files")
	}
	ymlFileList, err := f.FetchByPattern(filepath.Join(capiutils.ManifestDir, "*.yml"))
	if err != nil {
		return false, errors.Wrap(err, "failed to load *.yml files")
	}
	jsonFileList, err := f.FetchByPattern(filepath.Join(capiutils.ManifestDir, "*.json"))
	if err != nil {
		return false, errors.Wrap(err, "failed to load *.json files")
	}
	fileList := append(yamlFileList, ymlFileList...) //nolint:gocritic
	fileList = append(fileList, jsonFileList...)

	for _, file := range fileList {
		decoded, err := clusterapi.ObjectsFromManifest(file.Filename, file.Data)
		if err != nil {
			return false, err
		}
		for i, d := range decoded {
			name := file.Filename
			if len(decoded) > 1 {
				ext := filepath.Ext(name)
				name = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(name, ext), i+1, ext)
			}
			c.FileList = append(c.FileList, &asset.RuntimeFile{
				File:   asset.File{Filename: name, Data: d.Data},
				Object: d.Object,
			})
		}
	}

	if len(c.FileList) == 0 {
		return false, nil
	}

	// Generate always emits the guests namespace, and Provision creates every
	// manifest into it (pkg/infrastructure/clusterapi/clusterapi.go). Loading
	// from disk replaces the generated list wholesale, so without this the
	// namespace is simply absent and the first create fails with
	// `namespaces "openshift-cluster-api-guests" not found`. Users are not
	// told to write a Namespace manifest, so the invariant is restored here
	// rather than required of them.
	if !hasNamespace(c.FileList) {
		nsFile, err := namespaceRuntimeFile()
		if err != nil {
			return false, err
		}
		c.FileList = append(c.FileList, nsFile)
	}

	asset.SortManifestFiles(c.FileList)
	return true, nil
}

// hasNamespace reports whether the list already contains the guests namespace,
// so that a user who does supply one does not get a duplicate that would fail
// to create.
//
// The kind is taken from the Go type, not from TypeMeta. Every object this is
// ever asked about came out of clusterapi.ObjectsFromManifest, and a typed
// object produced by Scheme.Convert has an empty TypeMeta -- so asking
// GetObjectKind() reports Kind="" for a Namespace that is plainly there, this
// function returns false, a second namespace is appended, and Provision fails
// with `namespaces "openshift-cluster-api-guests" already exists`. That is how
// a second `openshift-install create cluster` over an already-generated
// manifest directory used to die.
//
// Only a kind the installer has no type for stays unstructured, and for those
// TypeMeta is the only thing there is, so both cases are handled.
func hasNamespace(files []*asset.RuntimeFile) bool {
	for _, f := range files {
		if f.Object == nil || f.Object.GetName() != capiutils.Namespace {
			continue
		}
		switch o := f.Object.(type) {
		case *corev1.Namespace:
			return true
		case *unstructured.Unstructured:
			gvk := o.GroupVersionKind()
			if gvk.Group == "" && gvk.Kind == "Namespace" {
				return true
			}
		}
	}
	return false
}

// namespaceRuntimeFile builds the guests namespace manifest, matching what
// Generate emits.
func namespaceRuntimeFile() (*asset.RuntimeFile, error) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: capiutils.Namespace},
	}
	namespace.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))
	data, err := yaml.Marshal(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the %s namespace: %w", capiutils.Namespace, err)
	}
	return &asset.RuntimeFile{
		File:   asset.File{Filename: filepath.Join(capiutils.ManifestDir, "000_capi-namespace.yaml"), Data: data},
		Object: namespace,
	}, nil
}
