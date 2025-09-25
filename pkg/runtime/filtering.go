/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package runtime

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/nerdctl/pkg/imgutil"
	"github.com/containerd/nerdctl/pkg/referenceutil"
)

// Filter types supported to filter images.
var (
	FilterBeforeType    = "before"
	FilterSinceType     = "since"
	FilterLabelType     = "label"
	FilterReferenceType = "reference"
	FilterDanglingType  = "dangling"
	FilterSizeType      = "size"
)

// Filters contains all types of filters to filter images.
type Filters struct {
	Before    []string
	Since     []string
	Labels    map[string]string
	Reference []string
	Dangling  *bool
	Size      []string
}

// ParseFilters parse filter strings.
func ParseFilters(filters []string) (*Filters, error) {
	f := &Filters{Labels: make(map[string]string)}
	for _, filter := range filters {
		tempFilterToken := strings.Split(filter, "=")
		switch len(tempFilterToken) {
		case 1:
			return nil, fmt.Errorf("invalid filter %q", filter)
		case 2:
			if tempFilterToken[0] == FilterDanglingType {
				var isDangling bool
				if tempFilterToken[1] == "true" {
					isDangling = true
				} else if tempFilterToken[1] == "false" {
					isDangling = false
				} else {
					return nil, fmt.Errorf("invalid filter %q", filter)
				}
				f.Dangling = &isDangling
			} else if tempFilterToken[0] == FilterBeforeType {
				canonicalRef, err := referenceutil.ParseAny(tempFilterToken[1])
				if err != nil {
					return nil, err
				}

				f.Before = append(f.Before, fmt.Sprintf("name==%s", canonicalRef.String()))
				f.Before = append(f.Before, fmt.Sprintf("name==%s", tempFilterToken[1]))
			} else if tempFilterToken[0] == FilterSinceType {
				canonicalRef, err := referenceutil.ParseAny(tempFilterToken[1])
				if err != nil {
					return nil, err
				}
				f.Since = append(f.Since, fmt.Sprintf("name==%s", canonicalRef.String()))
				f.Since = append(f.Since, fmt.Sprintf("name==%s", tempFilterToken[1]))
			} else if tempFilterToken[0] == FilterLabelType {
				// To support filtering labels by keys.
				f.Labels[tempFilterToken[1]] = ""
			} else if tempFilterToken[0] == FilterReferenceType {
				f.Reference = append(f.Reference, tempFilterToken[1])
			} else if tempFilterToken[0] == FilterSizeType {
				f.Size = append(f.Size, tempFilterToken[1])
			} else {
				return nil, fmt.Errorf("invalid filter %q", filter)
			}
		case 3:
			if tempFilterToken[0] == FilterLabelType {
				f.Labels[tempFilterToken[1]] = tempFilterToken[2]
			} else {
				return nil, fmt.Errorf("invalid filter %q", filter)
			}
		default:
			return nil, fmt.Errorf("invalid filter %q", filter)
		}
	}
	return f, nil
}

// FilterImages returns images in `labelImages` that are created
// before MAX(beforeImages.CreatedAt) and after MIN(sinceImages.CreatedAt).
func FilterImages(labelImages []images.Image, beforeImages []images.Image, sinceImages []images.Image) []images.Image {
	return imgutil.FilterImages(labelImages, beforeImages, sinceImages)
}

// FilterByReference filters images using references given in `filters`.
func FilterByReference(imageList []images.Image, filters []string) ([]images.Image, error) {
	return imgutil.FilterByReference(imageList, filters)
}

// FilterDangling filters dangling images (or keeps if `dangling` == false).
func FilterDangling(imageList []images.Image, dangling bool) []images.Image {
	return imgutil.FilterDangling(imageList, dangling)
}

// FilterByLabel filters images based on labels given in `filters`.
func FilterByLabel(ctx context.Context, client *containerd.Client, imageList []images.Image, filters map[string]string) ([]images.Image, error) {
	return imgutil.FilterByLabel(ctx, client, imageList, filters)
}

// FilterBySize filters images based on size conditions given in `filters`.
// TODO: cache parsed filters
func FilterBySize(ctx context.Context, client *containerd.Client, snapshotter snapshots.Snapshotter, imageList []images.Image, filters []string) ([]images.Image, error) {
	var filteredImages []images.Image
	// Filter the images based on the size
	for _, img := range imageList {
		matched := true
		clientImage := containerd.NewImage(client, img)
		imgSize, err := imgutil.UnpackedImageSize(ctx, snapshotter, clientImage)
		if err != nil {
			return nil, err
		}
		for _, filter := range filters {
			operator, size, err := parseSizeFilter(filter)
			if err != nil {
				return nil, err
			}
			if !matchesSizeFilter(imgSize, operator, size) {
				matched = false
			}
		}
		if matched {
			filteredImages = append(filteredImages, img)
		}
	}
	return filteredImages, nil
}

// parseSizeFilter parses a filter value like ">=10MB" or "<512MiB" into an operator and size in bytes.
// Supported operators: >, >=, <, <=, =, ==
// Supported units (case-insensitive): B, KB, KiB, MB, MiB, GB, GiB, TB, TiB. If omitted, bytes are assumed.
func parseSizeFilter(val string) (string, int64, error) {
	s := strings.TrimSpace(val)
	if s == "" {
		return "", 0, fmt.Errorf("empty size filter")
	}
	// detect operator (check 2-char ops first)
	var op string
	for _, candidate := range []string{">=", "<=", "==", ">", "<", "="} {
		if strings.HasPrefix(s, candidate) {
			op = candidate
			s = strings.TrimSpace(s[len(candidate):])
			break
		}
	}
	if op == "" {
		return "", 0, fmt.Errorf("invalid size filter, operator missing: %q", val)
	}
	if s == "" {
		return "", 0, fmt.Errorf("invalid size filter, missing number: %q", val)
	}
	// split numeric and unit
	i := 0
	dotSeen := false
	for i < len(s) {
		c := s[i]
		if (c >= '0' && c <= '9') || (c == '.' && !dotSeen) {
			if c == '.' {
				dotSeen = true
			}
			i++
			continue
		}
		break
	}
	if i == 0 {
		return "", 0, fmt.Errorf("invalid size filter, expected number: %q", val)
	}
	numStr := s[:i]
	unitStr := strings.TrimSpace(s[i:])
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid size number %q: %w", numStr, err)
	}
	// resolve unit multiplier
	mult := float64(1)
	u := strings.ToLower(unitStr)
	switch u {
	case "", "b":
		mult = 1
	case "kb":
		mult = 1e3
	case "kib":
		mult = 1024
	case "mb":
		mult = 1e6
	case "mib":
		mult = 1024 * 1024
	case "gb":
		mult = 1e9
	case "gib":
		mult = 1024 * 1024 * 1024
	case "tb":
		mult = 1e12
	case "tib":
		mult = 1024 * 1024 * 1024 * 1024
	default:
		return "", 0, fmt.Errorf("invalid size unit %q", unitStr)
	}
	size := int64(num * mult)
	return op, size, nil
}

// matchesSizeFilter compares an image size to a size filter operator and value.
func matchesSizeFilter(imgSize int64, operator string, size int64) bool {
	switch operator {
	case ">":
		return imgSize > size
	case ">=":
		return imgSize >= size
	case "<":
		return imgSize < size
	case "<=":
		return imgSize <= size
	case "=", "==":
		return imgSize == size
	default:
		// unknown operator, treat as no match
		return false
	}
}
