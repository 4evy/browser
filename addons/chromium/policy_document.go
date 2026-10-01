package chromium

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/4evy/browser/core"
	"github.com/4evy/browser/internal/fileutil"
)

// MergePolicyDocuments combines process-wide policy intent and rejects
// conflicts
func MergePolicyDocuments(documents ...map[string]any) (map[string]any, error) {
	return core.MergePolicies(documents...)
}

// EncodeManagedPolicy writes a deterministic Chromium policy document
func EncodeManagedPolicy(writer io.Writer, policies map[string]any) error {
	var data bytes.Buffer
	err := json.MarshalEncode(
		jsontext.NewEncoder(&data, jsontext.WithIndent("  ")),
		policies,
		json.Deterministic(true),
	)
	if err != nil {
		return fmt.Errorf("encode managed policy: %w", err)
	}
	if _, err := writer.Write(data.Bytes()); err != nil {
		return fmt.Errorf("write managed policy: %w", err)
	}
	return nil
}

// WriteManagedPolicyFile atomically writes a deterministic Chromium
// managed-policy JSON file
func WriteManagedPolicyFile(path string, policies map[string]any) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("managed policy output path is required")
	}
	if _, err := fileutil.WriteJSONIfChanged(
		path,
		policies,
		fileutil.DefaultFilePerm,
	); err != nil {
		return fmt.Errorf("write managed policy %s: %w", path, err)
	}
	return nil
}
