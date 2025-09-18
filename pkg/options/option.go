package options

type RebaseOptions struct {
	RootOptions
	ImageRef        string `json:"image_ref"`
	NewImageName    string `json:"new_image_name"`
	BaseLayerDigest string `json:"base_layer_digest"`
	NewBaseImageRef string `json:"new_base_image_ref"`
	AutoSquash      bool   `json:"auto_squash"`
}

type RemoveOptions struct {
	RootOptions
	File         string `json:"file"`
	ImageRef     string `json:"image_ref"`
	NewImageName string `json:"new_image_name"`
}

type VerifyBaseOptions struct {
	RootOptions
	OriginalImage string `json:"original_image"`
	BaseImage     string `json:"base_image"`
}

type SearchHistoryOptions struct {
	RootOptions
	ImageRef string `json:"image_ref"`
	Keyword  string `json:"keyword"`
	Format   string `json:"format"`
	Quiet    bool   `json:"quiet"`
	NoTrunc  bool   `json:"no_trunc"`
}

type RootOptions struct {
	ContainerdAddress string `json:"containerd_address"`
	Namespace         string `json:"namespace"`
	LogLevel          string `json:"log_level"`
}
