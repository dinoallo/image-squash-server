# Image Manip CLI

This project provides CLI tools for manipulating container images using containerd. The main commands are `rebase` and `remove`.

There is also an internal verification helper (`verify-base`) used to confirm that one image is derived from a specified base image by checking the prefix of its layer chain.


## Rebase Logic

The `rebase` command replaces the base layers of an image with those from a new base image, while preserving the application (non‑base) layers.

Default behavior (no squashing):
By default every application layer is retained as-is (no layer squashing). This keeps history granular and avoids unnecessary rebuild cost when only metadata changes. Previously the tool squashed all application layers into one; that behavior is now optional via `--auto-squash`.

Process steps:
1. Fetch the original image, the (old) base image, and the new base image.
2. Verify the original image is based on the specified base image (layer digest prefix match).
3. Identify the application layers to rebase (those above the old base layer count).
4. (Optional) If `--auto-squash` is set, all application layers are treated as one squash group except the first (git-rebase style: first `pick`, rest `fixup`). Otherwise all are individually `pick`ed.
5. Generate a new image config & manifest combining the new base layers and (possibly squashed) application layers.
6. Write new image contents and update/create the target image reference.
7. Unpack the resulting image for immediate use.

Why keep layers separate by default?
* Faster incremental distribution (only changed layers are pushed/pulled).
* Easier debugging/auditing of layer contents.
Use `--auto-squash` when you explicitly want a single compact application layer (e.g., to reduce metadata noise or for proprietary distribution).


## Remove Logic

The `remove` command creates a new image by removing a specified file from the original image. The process involves:

1. Fetching the original image.
2. Creating a temporary root filesystem from the image layers.
3. Mounting the root filesystem and removing the target file.
4. Creating a new layer representing the file removal.
5. Generating a new image config and manifest with the new layer.
6. Writing the new image contents and updating the image reference.
7. Unpacking the new image for use.

## Verify-Base Logic

The `verify-base` logic (implemented in `Runtime.Verifybase`) checks whether a given image was built on top of an expected base image. It works by:

1. Loading both the candidate (original) image and the claimed base image.
2. Comparing the sequence of layer digests of the base image with the first N layers of the candidate image (where N is the number of base layers).
3. Failing if the base has more layers than the candidate or if any digest mismatch occurs.
4. Succeeding if all base layer digests match in order, meaning the candidate image is based on the provided base.

If successful, the system logs a confirmation message; otherwise it returns an error detailing the mismatch cause.

## Commands

### `rebase`
Rebase a container image onto a new base image.

**Usage:**
```
rebase ORIGINAL_IMAGE NEW_BASE_IMAGE [flags]
```

- `ORIGINAL_IMAGE`: The reference to the original image to be rebased.
- `NEW_BASE_IMAGE`: The reference to the new base image.

**Flags:**
- `--containerd-address`: containerd address (default: `unix:///var/run/containerd/containerd.sock`)
- `--namespace`: containerd namespace (default: `k8s.io`)
- `--base-image`: old base image ref, if not specified, will be the same as the original image
- `--new-image`: new image ref, if not specified, will be the same as the original image
- `--auto-squash`: squash all application layers above the base into a single layer (disabled by default)

### `remove`
Remove a file from a container image.

**Usage:**
```
remove FILE IMAGE [flags]
```

- `FILE`: Path to the file to remove from the image.
- `IMAGE`: The reference to the image to modify.

**Flags:**
- `--containerd-address`: containerd address (default: `unix:///var/run/containerd/containerd.sock`)
- `--namespace`: containerd namespace (default: `k8s.io`)
- `--new-image`: new image ref, if not specified, will be the same as the original image

## Example

Rebase an image:
```
rebase my-app:latest ubuntu:22.04 --new-image my-app-rebased:latest
```

Rebase and squash all application layers into one (previous default behavior):
```
rebase my-app:latest ubuntu:22.04 --new-image my-app-rebased:latest --auto-squash
```

Remove a file from an image:
```
remove /etc/secret.conf my-app:latest --new-image my-app-no-secret:latest
```

## Requirements
- Go
- containerd

## License
MIT
