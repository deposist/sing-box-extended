package rule

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
)

// writeBinaryRuleSet writes a valid binary rule-set and returns its path.
func writeBinaryRuleSet(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.srs")
	file, err := os.Create(path)
	require.NoError(t, err)
	err = srs.Write(file, option.PlainRuleSet{
		Rules: []option.HeadlessRule{
			{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultHeadlessRule{
					Domain: []string{"example.com"},
				},
			},
		},
	}, C.RuleSetVersionCurrent)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	return path
}

func localBinaryRuleSetOptions(path string) option.RuleSet {
	return option.RuleSet{
		Type:   C.RuleSetTypeLocal,
		Format: C.RuleSetFormatBinary,
		LocalOptions: option.LocalRuleSet{
			Path: path,
		},
	}
}

// TestLocalRuleSetBinaryReleasesFileOnSuccess proves the .srs handle is not
// retained after a successful load, so the file can be replaced immediately.
// On Windows a leaked handle makes the rename/remove below fail. No retries.
func TestLocalRuleSetBinaryReleasesFileOnSuccess(t *testing.T) {
	path := writeBinaryRuleSet(t)

	for i := 0; i < 8; i++ {
		ruleSet, err := NewLocalRuleSet(context.Background(), log.NewNOPFactory().Logger(), "test", localBinaryRuleSetOptions(path))
		require.NoError(t, err)
		require.NoError(t, ruleSet.Close())

		renamed := path + ".renamed"
		require.NoErrorf(t, os.Rename(path, renamed), "rename after successful load, iteration %d", i)
		require.NoErrorf(t, os.Rename(renamed, path), "rename back, iteration %d", i)
	}

	require.NoError(t, os.Remove(path))
}

// TestLocalRuleSetBinaryReleasesFileOnParseError proves the .srs handle is not
// retained when decoding fails, so the malformed file can be replaced
// immediately. No retries.
func TestLocalRuleSetBinaryReleasesFileOnParseError(t *testing.T) {
	valid := writeBinaryRuleSet(t)
	content, err := os.ReadFile(valid)
	require.NoError(t, err)
	require.Greater(t, len(content), 8)

	path := filepath.Join(t.TempDir(), "truncated.srs")
	// Keep the magic bytes and version, truncate the compressed payload so
	// rule decoding fails partway through.
	require.NoError(t, os.WriteFile(path, content[:len(content)/2], 0o600))

	for i := 0; i < 8; i++ {
		_, err := NewLocalRuleSet(context.Background(), log.NewNOPFactory().Logger(), "test", localBinaryRuleSetOptions(path))
		require.Errorf(t, err, "expected decode error, iteration %d", i)

		renamed := path + ".renamed"
		require.NoErrorf(t, os.Rename(path, renamed), "rename after failed load, iteration %d", i)
		require.NoErrorf(t, os.Rename(renamed, path), "rename back, iteration %d", i)
	}

	require.NoError(t, os.Remove(path))
}
