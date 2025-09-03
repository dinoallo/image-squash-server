# Image Manip CLI

This project provides CLI tools for manipulating container images using containerd. The main commands are `rebase` and `remove`.

There is also an internal verification helper (`verify-base`) used to confirm that one image is derived from a specified base image by checking the prefix of its layer chain.


## Rebase Logic

The `rebase` command replaces the base layers of an image with those from a new base image, while preserving the application layers. The process involves:

1. Fetching the original image, base image, and new base image.
2. Verifying the original image is based on the specified base image.
3. Identifying layers to rebase (application layers above the base).
4. Creating a new image config and manifest using the new base image and rebased layers.
5. Writing the new image contents and updating the image reference.
6. Unpacking the new image for use.


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

Remove a file from an image:
```
remove /etc/secret.conf my-app:latest --new-image my-app-no-secret:latest
```

## Requirements
- Go
- containerd

## License
MIT
