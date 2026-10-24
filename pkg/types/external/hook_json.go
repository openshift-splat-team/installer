package external

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// UnmarshalJSON reads a Hook from either the object form or a bare string.
//
// The object form is the one to write:
//
//	postProvision:
//	  program: hooks/dns.sh
//	  args: ["--input-dns-zone=Z123"]
//
// A bare string is accepted as shorthand for an object with no arguments,
// `preDestroy: hooks/dns.sh`, and that is not only a convenience. It is a
// compatibility requirement, because a Hook is not read only from the
// install-config. It is recorded in metadata.json when the cluster is created
// and read back by `destroy cluster` -- necessarily by a later build of the
// installer, and possibly by a much later one. Hooks were a plain string
// before they carried arguments, so every install directory written by an
// earlier build has the string form on disk. Without this, upgrading the
// installer would make those clusters undestroyable: the teardown hook that
// knows how to remove their DNS could not be read, and the resources it
// created would leak with nothing left to identify them.
//
// Marshalling is deliberately not customised. Everything written from now on
// is written in the object form, so the string form only ever shrinks as a
// population.
func (h *Hook) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var program string
		if err := json.Unmarshal(trimmed, &program); err != nil {
			return fmt.Errorf("hook is neither a program path nor a hook object: %w", err)
		}
		*h = Hook{Program: program}
		return nil
	}

	// A local type without methods, so unmarshalling it does not call this
	// function again.
	type plain Hook
	var out plain
	if err := json.Unmarshal(trimmed, &out); err != nil {
		return err
	}
	*h = Hook(out)
	return nil
}
