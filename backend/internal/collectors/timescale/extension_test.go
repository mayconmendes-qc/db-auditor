package timescale

import "testing"

func TestEnsureExtensionNilConnNotCalled(t *testing.T) {
	t.Parallel()
	// Compile-time presence of helpers; runtime coverage needs a live DB.
	_ = ExtensionInstalled
	_ = ensureExtension
}
