package options

type RebaseOptions struct {
	RootOptions
	OriginalImage string `json:"original_image"`
	NewImage      string `json:"new_image"`
	AutoSquash    bool   `json:"auto_squash"`
	BaseLayer     string `json:"base_layer"`
	NewBaseImage  string `json:"new_base_image"`
}

type RemoveOptions struct {
	RootOptions
	File          string `json:"file"`
	OriginalImage string `json:"original_image"`
	NewImage      string `json:"new_image"`
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
