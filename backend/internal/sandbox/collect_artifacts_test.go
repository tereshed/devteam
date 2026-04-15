package sandbox

import (
	"archive/tar"
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/require"
)

func TestReadSingleRegularFromDockerTar_readsFirstMatchingReg(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "workspace/other.txt",
		Mode:     0o644,
		Typeflag: tar.TypeReg,
		Size:     3,
	}))
	_, err := tw.Write([]byte("abc"))
	require.NoError(t, err)
	statusBody := []byte(`{"ok":true}`)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "workspace/status.json",
		Mode:     0o644,
		Typeflag: tar.TypeReg,
		Size:     int64(len(statusBody)),
	}))
	_, err = tw.Write(statusBody)
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	got, found, err := readSingleRegularFromDockerTar(&buf, "workspace/status.json", CodeResultMaxArtifactBytes)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, `{"ok":true}`, string(got))
}

func TestReadSingleRegularFromDockerTar_skipsSymlinkWithoutFollowing(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "link",
		Mode:     0o777,
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
		Size:     0,
	}))
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "workspace/status.json",
		Mode:     0o644,
		Typeflag: tar.TypeReg,
		Size:     2,
	}))
	_, err := tw.Write([]byte("42"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	got, found, err := readSingleRegularFromDockerTar(&buf, "workspace/status.json", CodeResultMaxArtifactBytes)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "42", string(got))
}

func TestReadSingleRegularFromDockerTar_truncatesAndDrainsLargeFile(t *testing.T) {
	const total = int64(CodeResultMaxArtifactBytes + 5000)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	payload := bytes.Repeat([]byte("x"), int(total))
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "workspace/full.diff",
		Mode:     0o644,
		Typeflag: tar.TypeReg,
		Size:     total,
	}))
	_, err := tw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	got, found, err := readSingleRegularFromDockerTar(bytes.NewReader(buf.Bytes()), "workspace/full.diff", CodeResultMaxArtifactBytes)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, got, CodeResultMaxArtifactBytes)
}

func TestReadSingleRegularFromDockerTar_basenameOnly(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "status.json",
		Mode:     0o644,
		Typeflag: tar.TypeReg,
		Size:     4,
	}))
	_, err := tw.Write([]byte("hey!"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	got, found, err := readSingleRegularFromDockerTar(&buf, "workspace/status.json", CodeResultMaxArtifactBytes)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "hey!", string(got))
}

func TestParseStatusJSON_trimsWhitespaceBranchAndCommit(t *testing.T) {
	raw := []byte(`{"success":true,"exit_code":0,"branch_name":"  x  ","commit_hash":"   "}`)
	doc, err := parseStatusJSON(raw)
	require.NoError(t, err)
	require.True(t, doc.Success)
	cr := &CodeResult{}
	applyStatusJSONToCodeResult(cr, doc)
	require.Equal(t, "x", cr.BranchName)
	require.Equal(t, "", cr.CommitHash)
}

func TestParseStatusJSON_invalid(t *testing.T) {
	_, err := parseStatusJSON([]byte(`{`))
	require.Error(t, err)
}

func TestMergeArtifactResultsIntoFinalStatus_contractMissingDowngradesCompleted(t *testing.T) {
	st := newInstanceState("task-1")
	fs := &SandboxStatus{ID: "c1", Status: SandboxStatusCompleted, ExitCode: 0}
	insp := &types.ContainerJSON{
		ContainerJSONBase: &ctypes.ContainerJSONBase{
			State: &ctypes.State{OOMKilled: false},
		},
	}
	out := &artifactCollectionOutcome{StatusJSONMissing: true}
	mergeArtifactResultsIntoFinalStatus(fs, st, insp, out, false)
	require.Equal(t, SandboxStatusFailed, fs.Status)
	require.NotNil(t, fs.Result)
	require.False(t, fs.Result.Success)
}

func TestMergeArtifactResultsIntoFinalStatus_timedOutKeepsStatus(t *testing.T) {
	st := newInstanceState("task-1")
	st.mu.Lock()
	st.businessTimeoutIntent = true
	st.mu.Unlock()
	doc := &statusJSONDoc{Success: true}
	out := &artifactCollectionOutcome{StatusJSONOK: true, parsed: doc}
	fs := &SandboxStatus{ID: "c1", Status: SandboxStatusTimedOut, ExitCode: 137}
	insp := &types.ContainerJSON{
		ContainerJSONBase: &ctypes.ContainerJSONBase{
			State: &ctypes.State{OOMKilled: false},
		},
	}
	mergeArtifactResultsIntoFinalStatus(fs, st, insp, out, true)
	require.Equal(t, SandboxStatusTimedOut, fs.Status)
	require.True(t, fs.Result.Success)
}

func TestCodeResultLogValueDoesNotLeakSecretSubstring(t *testing.T) {
	secret := "sk-ant-api03-super-secret-do-not-log-this-token"
	cr := &CodeResult{
		Success: true,
		Output:  strings.Repeat("a", 300) + secret,
		Diff:    "diff",
	}
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.New(h).Info("sandbox", "result", cr)
	out := buf.String()
	if strings.Contains(out, secret) {
		t.Fatalf("secret leaked into slog output: %s", out)
	}
}
