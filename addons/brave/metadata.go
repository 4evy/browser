package brave

import (
	_ "embed"

	addonmetadata "github.com/4evy/browser/addons/metadata"
)

//go:embed metadata.json
var embeddedMetadata []byte

var metadata = addonmetadata.MustLoad(embeddedMetadata, "brave")
