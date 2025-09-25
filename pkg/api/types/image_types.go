package types

import (
	"io"

	"github.com/lingdie/image-manip-server/pkg/options"
)

// ImageListOptions specifies options for `nerdctl image list`.
type ImageListOptions struct {
	options.RootOptions
	Stdout io.Writer
	// Quiet only show numeric IDs
	Quiet bool
	// NoTrunc don't truncate output
	NoTrunc bool
	// Format the output using the given Go template, e.g, '{{json .}}', 'wide'
	Format string
	// Filter output based on conditions provided, for the --filter argument
	Filters []string
	// NameAndRefFilter filters images by name and reference
	NameAndRefFilter []string
	// Digests show digests (compatible with Docker, unlike ID)
	Digests bool
	// Names show image names
	Names bool
	// All (unimplemented yet, always true)
	All bool
	// SortBy specifies the sort key
	SortBy string
}
