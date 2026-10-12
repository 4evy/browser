package cli

import (
	_ "embed"
	"strings"
)

//go:embed examples/examples.sh
var rootExample string

//go:embed examples/apply.sh
var applyExample string

//go:embed examples/config-validate.sh
var configValidateExample string

//go:embed examples/policy-render.sh
var policyRenderExample string

func commandExample(script string) string {
	return "  " + strings.ReplaceAll(strings.TrimSuffix(script, "\n"), "\n", "\n  ")
}
