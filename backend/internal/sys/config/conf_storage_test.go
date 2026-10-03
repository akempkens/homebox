package config

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"
)

const rootFileURL = "file:///"

func Test_Storage_withoutRootBucket(t *testing.T) {
	tests := []struct {
		name string
		in   Storage
		want Storage
	}{
		{"docker default is rerooted at the prefix dir",
			Storage{ConnString: "file:///?no_tmp_dir=true", PrefixPath: "data"},
			Storage{ConnString: "file:///data?no_tmp_dir=true", PrefixPath: ""}},
		{"nested prefix with stray slashes",
			Storage{ConnString: rootFileURL, PrefixPath: "/srv/homebox/"},
			Storage{ConnString: "file:///srv/homebox", PrefixPath: ""}},
		{"backslash prefix",
			Storage{ConnString: rootFileURL, PrefixPath: `srv\homebox`},
			Storage{ConnString: "file:///srv/homebox", PrefixPath: ""}},
		{"dot prefix resolves to root and is left alone",
			Storage{ConnString: rootFileURL, PrefixPath: "."},
			Storage{ConnString: rootFileURL, PrefixPath: "."}},
		{"root with empty prefix is left alone",
			Storage{ConnString: rootFileURL, PrefixPath: ""},
			Storage{ConnString: rootFileURL, PrefixPath: ""}},
		{"relative default is untouched",
			Storage{ConnString: "file:///./", PrefixPath: ".data"},
			Storage{ConnString: "file:///./", PrefixPath: ".data"}},
		{"absolute non-root dir is untouched",
			Storage{ConnString: "file:///data?no_tmp_dir=true", PrefixPath: "x"},
			Storage{ConnString: "file:///data?no_tmp_dir=true", PrefixPath: "x"}},
		{"windows drive path is untouched",
			Storage{ConnString: "file:///C:/homebox", PrefixPath: "data"},
			Storage{ConnString: "file:///C:/homebox", PrefixPath: "data"}},
		{"s3 with prefix is untouched",
			Storage{ConnString: "s3://bucket?region=eu-central-1", PrefixPath: "data"},
			Storage{ConnString: "s3://bucket?region=eu-central-1", PrefixPath: "data"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.withoutRootBucket())
		})
	}
}

// Regression: gocloud.dev v0.46.0 fileblob rejects every key as "escapes bucket
// root" when the bucket dir is "/". The Docker default (file:///?no_tmp_dir=true
// + prefix "data") must still be able to read an attachment after normalization.
func Test_Storage_withoutRootBucket_attachmentIsReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("root-relative file URLs differ on Windows")
	}
	dir := t.TempDir()
	docs := filepath.Join(dir, "gid", "documents")
	require.NoError(t, os.MkdirAll(docs, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(docs, "h"), []byte("ok"), 0o644))

	s := Storage{
		ConnString: "file:///?no_tmp_dir=true",
		PrefixPath: strings.TrimPrefix(filepath.ToSlash(dir), "/"),
	}.withoutRootBucket()

	ctx := context.Background()
	b, err := blob.OpenBucket(ctx, s.ConnString)
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Close() })

	// The attachment repo builds keys as <prefix>/<gid>/documents/<hash>; with the
	// prefix folded into the bucket dir the key is just the relative part.
	got, err := b.ReadAll(ctx, "gid/documents/h")
	require.NoError(t, err)
	assert.Equal(t, "ok", string(got))
}
