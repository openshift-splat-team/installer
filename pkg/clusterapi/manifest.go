package clusterapi

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// ObjectsFromManifest decodes a user-supplied Cluster API manifest into
// client.Objects, one per YAML document.
//
// A document whose GroupVersionKind is compiled into the installer's scheme is
// decoded into its Go type, as it always has been. One whose kind the
// installer does not know is carried through as an *unstructured.Unstructured,
// which also satisfies client.Object: the CRDs for a user-supplied
// infrastructure provider are installed into the local control plane from the
// provider's own components, so the API server can validate and persist an
// object whose type this binary was never compiled against.
//
// Unresolved objects are not accepted implicitly. An *unstructured.Unstructured
// reaching Provision is rejected there unless the platform's provider opts in
// by implementing infrastructure/clusterapi.UnstructuredManifestTolerator. That
// keeps the strict, load-time-equivalent check for every compiled-in platform:
// a misspelled kind in an integrated provider's manifest still stops the
// install before any cloud resource is created.
//
// Multiple documents separated by `---` are each decoded. The natural way to
// supply a Cluster and its infrastructure object is one file containing both,
// and sigs.k8s.io/yaml.Unmarshal silently keeps only the first document, so
// splitting here is what makes that file mean what it looks like it means.
//
// filename is used only for diagnostics.
func ObjectsFromManifest(filename string, data []byte) ([]DecodedManifest, error) {
	var out []DecodedManifest

	reader := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(data)))
	for i := 0; ; i++ {
		doc, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", filename, err)
		}
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		obj, err := objectFromDocument(filename, i, doc)
		if err != nil {
			return nil, err
		}
		out = append(out, DecodedManifest{Object: obj, Data: doc})
	}

	return out, nil
}

// DecodedManifest is one YAML document from a user-supplied manifest file,
// together with the bytes it was decoded from.
//
// The bytes are kept per-document rather than per-file so that a file holding
// several documents round-trips to one asset file per object instead of
// repeating the whole file under each.
type DecodedManifest struct {
	Object client.Object
	Data   []byte
}

// objectFromDocument decodes a single YAML document. index is the
// zero-based position of the document in the file, used only for diagnostics.
func objectFromDocument(filename string, index int, doc []byte) (client.Object, error) {
	where := filename
	if index > 0 {
		where = fmt.Sprintf("%s (document %d)", filename, index+1)
	}

	u := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(doc, u); err != nil {
		// %s, not %w: nothing downstream may unwrap this back to the
		// original error and print the payload it carries.
		return nil, fmt.Errorf("failed to unmarshal %s: %s", where, redactDecodeError(err))
	}

	// A document with no kind is already rejected by the unmarshal above:
	// unstructured.Unstructured requires it.
	gvk := u.GroupVersionKind()
	obj, err := Scheme.New(gvk)
	if err != nil {
		logrus.Debugf("Manifest %s declares kind %s, which this installer has no type for; "+
			"carrying it through unresolved.", where, gvk)
		return u, nil
	}

	if err := Scheme.Convert(u, obj, nil); err != nil {
		return nil, fmt.Errorf("failed to convert %s: %w", where, err)
	}
	co, ok := obj.(client.Object)
	if !ok {
		return nil, fmt.Errorf("%s declares kind %s, which is not a Kubernetes object", where, gvk)
	}
	return co, nil
}

// maxDecodeErrorLength bounds a decode error even after the known content-
// bearing forms have been trimmed, so an error shape not anticipated here
// still cannot spill a manifest into the log.
const maxDecodeErrorLength = 200

// redactDecodeError renders a decode failure without the document that caused
// it.
//
// k8s.io/apimachinery/pkg/runtime/error.go:90 and :112 format a missing kind
// or apiVersion as "Object 'Kind' is missing in '<the entire document>'". For
// an integrated provider that is merely noisy. For platform: external it is a
// disclosure: the manifests are the partner's own, this installer cannot know
// what they contain, and the message lands in the installer log and from there
// in a support bundle. A 27,000-character error carrying a whole AWSCluster was
// how this was found.
//
// The location of the fault is preserved -- the caller has already added the
// file and document number, and a YAML syntax error keeps its line number,
// which is what someone actually needs to fix it.
func redactDecodeError(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, " in '"); i >= 0 {
		msg = msg[:i]
	}
	if len(msg) > maxDecodeErrorLength {
		msg = msg[:maxDecodeErrorLength] + "... (truncated)"
	}
	return msg
}
