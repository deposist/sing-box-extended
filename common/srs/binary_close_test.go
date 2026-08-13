package srs

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
)

// trackingCloser counts Close calls and can report a close error.
type trackingCloser struct {
	io.Reader
	closes   int
	closeErr error
}

func (c *trackingCloser) Close() error {
	c.closes++
	return c.closeErr
}

// trackingDecompressor returns a decompressor factory that wraps zlib and
// records the tracking closer it produced.
func trackingDecompressor(closeErr error, out **trackingCloser) func(io.Reader) (io.ReadCloser, error) {
	return func(reader io.Reader) (io.ReadCloser, error) {
		zlibReader, err := zlib.NewReader(reader)
		if err != nil {
			return nil, err
		}
		closer := &trackingCloser{Reader: zlibReader, closeErr: closeErr}
		*out = closer
		return closer, nil
	}
}

func validBinaryRuleSet(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	err := Write(&buffer, option.PlainRuleSet{
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
	return buffer.Bytes()
}

func TestReadClosesDecompressorOnSuccess(t *testing.T) {
	var closer *trackingCloser
	ruleSet, err := read(bytes.NewReader(validBinaryRuleSet(t)), false, trackingDecompressor(nil, &closer))
	require.NoError(t, err)
	require.Len(t, ruleSet.Options.Rules, 1)
	require.NotNil(t, closer)
	require.Equal(t, 1, closer.closes, "decompressor must be closed exactly once on success")
}

func TestReadClosesDecompressorOnParseError(t *testing.T) {
	content := validBinaryRuleSet(t)
	var closer *trackingCloser
	_, err := read(bytes.NewReader(content[:len(content)/2]), false, trackingDecompressor(nil, &closer))
	require.Error(t, err)
	require.NotNil(t, closer)
	require.Equal(t, 1, closer.closes, "decompressor must be closed exactly once on parse failure")
}

func TestReadParseErrorTakesPrecedenceOverCloseError(t *testing.T) {
	content := validBinaryRuleSet(t)
	closeErr := errors.New("close failed")
	var closer *trackingCloser
	_, err := read(bytes.NewReader(content[:len(content)/2]), false, trackingDecompressor(closeErr, &closer))
	require.Error(t, err)
	require.NotErrorIs(t, err, closeErr, "parse error must take precedence over close error")
	require.NotNil(t, closer)
	require.Equal(t, 1, closer.closes)
}

func TestReadPropagatesCloseErrorWhenParseSucceeds(t *testing.T) {
	closeErr := errors.New("close failed")
	var closer *trackingCloser
	_, err := read(bytes.NewReader(validBinaryRuleSet(t)), false, trackingDecompressor(closeErr, &closer))
	require.ErrorIs(t, err, closeErr, "close error must surface when parsing succeeded")
	require.NotNil(t, closer)
	require.Equal(t, 1, closer.closes)
}

func TestReadRejectsCorruptOrTruncatedTrailer(t *testing.T) {
	var buffer bytes.Buffer
	require.NoError(t, Write(&buffer, option.PlainRuleSet{}, C.RuleSetVersionCurrent))
	valid := buffer.Bytes()

	corrupt := append([]byte(nil), valid...)
	corrupt[len(corrupt)-1] ^= 0xff
	_, err := Read(bytes.NewReader(corrupt), false)
	require.Error(t, err)

	_, err = Read(bytes.NewReader(valid[:len(valid)-1]), false)
	require.Error(t, err)
}

func TestReadRejectsTrailingRuleSetData(t *testing.T) {
	var compressed bytes.Buffer
	compressWriter := zlib.NewWriter(&compressed)
	_, err := compressWriter.Write([]byte{0, 1})
	require.NoError(t, err)
	require.NoError(t, compressWriter.Close())

	content := append([]byte(nil), MagicBytes[:]...)
	content = append(content, C.RuleSetVersionCurrent)
	content = append(content, compressed.Bytes()...)
	_, err = Read(bytes.NewReader(content), false)
	require.ErrorContains(t, err, "unexpected trailing rule-set data")
}
