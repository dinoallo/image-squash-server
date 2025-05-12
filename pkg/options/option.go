package options

type Option struct {
	SourceImage string `json:"source_image"`
	TargetImage string `json:"target_image"`

	SquashLayerCount  int    `json:"squash_layer_count"`
	SquashLayerDigest string `json:"squash_layer_digest"`
}
