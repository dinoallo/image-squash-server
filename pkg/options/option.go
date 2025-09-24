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

type HistoryOptions struct {
	RootOptions
	ImageRef string `json:"image_ref"`
	Format   string `json:"format"`
	Quiet    bool   `json:"quiet"`
	NoTrunc  bool   `json:"no_trunc"`
}

type SearchHistoryOptions struct {
	HistoryOptions
	Keyword string `json:"keyword"`
}

type RemoteOptions struct {
	RootOptions
	Insecure bool `json:"insecure"`
}

type RootOptions struct {
	ContainerdAddress string `json:"containerd_address"`
	Namespace         string `json:"namespace"`
	LogLevel          string `json:"log_level"`
}
