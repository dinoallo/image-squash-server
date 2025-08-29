package options

type SquashOption struct {
	SourceImage string `json:"source_image"`
	TargetImage string `json:"target_image"`

	SquashLayerCount  int    `json:"squash_layer_count"`
	SquashLayerDigest string `json:"squash_layer_digest"`
}

type RebaseOption struct {
	BaseImage     string `json:"base_image"`
	NewBaseImage  string `json:"new_base_image"`
	OriginalImage string `json:"original_image"`
	NewImage      string `json:"new_image"`
}

type EditOption struct {
	SourceImage    string   `json:"source_image"`
	FilesToRemoved []string `json:"files_to_removed"`
}

type RemoveOption struct {
	File          string `json:"file"`
	OriginalImage string `json:"original_image"`
	NewImage      string `json:"new_image"`
}
