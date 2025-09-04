package options

type RebaseOptions struct {
	RootOptions
	BaseImage     string `json:"base_image"`
	NewBaseImage  string `json:"new_base_image"`
	OriginalImage string `json:"original_image"`
	NewImage      string `json:"new_image"`
	AutoSquash    bool   `json:"auto_squash"`
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

type RootOptions struct {
	ContainerdAddress string `json:"containerd_address"`
	Namespace         string `json:"namespace"`
	LogLevel          string `json:"log_level"`
}
