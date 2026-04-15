package sandbox

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"

	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
)

// maxTarHeadersPerCopy — защита от tar-bomb / битого слоя (задача 5.7).
const maxTarHeadersPerCopy = 10000

// containerArtifactCopy — подмножество Docker API для юнит-тестов (мок CopyFromContainer).
// Docker Engine API v28+: CopyFromContainer без options, второй результат — PathStat (заголовки archive).
type containerArtifactCopy interface {
	CopyFromContainer(ctx context.Context, container, srcPath string) (io.ReadCloser, ctypes.PathStat, error)
}

// artifactCollectionOutcome — результат сбора после exited (5.7).
type artifactCollectionOutcome struct {
	StatusJSONOK      bool
	StatusJSONMissing bool
	BadJSON           bool

	parsed *statusJSONDoc

	diffText string
	logText  string
}

func (o *artifactCollectionOutcome) buildCodeResult() *CodeResult {
	if o == nil {
		return nil
	}
	cr := &CodeResult{
		// MVP 5.7 / вариант A: до артефакта git diff --name-only в entrypoint (5.2) — nil.
		FilesChanged: nil,
		Diff:         o.diffText,
		Output:       o.logText,
	}
	if o.StatusJSONOK && o.parsed != nil {
		applyStatusJSONToCodeResult(cr, o.parsed)
	} else {
		cr.Success = false
	}
	return cr
}

// collectSandboxArtifacts забирает status.json, full.diff, agent.log через CopyFromContainer (5.7).
// Ошибка возврата — только сбой Docker SDK при копировании; отсутствие файлов — в полях outcome.
func collectSandboxArtifacts(ctx context.Context, cli containerArtifactCopy, containerID string) (*artifactCollectionOutcome, error) {
	out := &artifactCollectionOutcome{}

	statusSuffix := strings.TrimPrefix(StatusJSONPath, "/")
	diffSuffix := strings.TrimPrefix(FullDiffPath, "/")
	logSuffix := strings.TrimPrefix(AgentLogPath, "/")

	stData, err, stNF := copyArtifactFile(ctx, cli, containerID, StatusJSONPath, statusSuffix, CodeResultMaxArtifactBytes)
	if err != nil {
		return nil, err
	}
	if stNF || len(stData) == 0 {
		out.StatusJSONMissing = true
	} else {
		doc, perr := parseStatusJSON(stData)
		if perr != nil {
			out.BadJSON = true
			slog.Debug("sandbox: status.json parse failed", "err", perr)
		} else {
			out.StatusJSONOK = true
			out.parsed = doc
		}
	}

	diffData, err, _ := copyArtifactFile(ctx, cli, containerID, FullDiffPath, diffSuffix, CodeResultMaxArtifactBytes)
	if err != nil {
		return nil, err
	}
	out.diffText = string(diffData)

	logData, err, _ := copyArtifactFile(ctx, cli, containerID, AgentLogPath, logSuffix, CodeResultMaxArtifactBytes)
	if err != nil {
		return nil, err
	}
	out.logText = string(logData)

	return out, nil
}

func copyArtifactFile(ctx context.Context, cli containerArtifactCopy, containerID, srcPath, nameSuffix string, maxBody int64) (_ []byte, err error, notFound bool) {
	rc, _, err := cli.CopyFromContainer(ctx, containerID, srcPath)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, nil, true
		}
		return nil, fmt.Errorf("copy from container: %w", errors.Join(ErrSandboxDocker, err)), false
	}
	defer func() { _ = rc.Close() }()

	data, found, rerr := readSingleRegularFromDockerTar(rc, nameSuffix, maxBody)
	if rerr != nil {
		return nil, fmt.Errorf("copy from container: unpack %s: %w", path.Base(srcPath), errors.Join(ErrSandboxDocker, rerr)), false
	}
	if !found {
		return nil, nil, true
	}
	return data, nil, false
}

func readSingleRegularFromDockerTar(r io.Reader, wantSuffix string, maxBody int64) (data []byte, found bool, err error) {
	tr := tar.NewReader(r)
	for i := 0; i < maxTarHeadersPerCopy; i++ {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			if !tarNameMatchesArtifact(hdr.Name, wantSuffix) {
				// Не читаем тело: следующий tr.Next() сам сбросит непрочитанный хвост записи (archive/tar).
				continue
			}
			body, err := readTarRegularLimited(tr, hdr.Size, maxBody)
			if err != nil {
				return nil, false, err
			}
			return body, true, nil
		default:
			continue
		}
	}
	return nil, false, fmt.Errorf("tar: exceeded %d headers", maxTarHeadersPerCopy)
}

func readTarRegularLimited(tr *tar.Reader, declaredSize, maxBody int64) ([]byte, error) {
	if declaredSize < 0 {
		return nil, fmt.Errorf("tar: negative file size")
	}
	limit := declaredSize
	if limit > maxBody {
		limit = maxBody
	}
	lr := io.LimitReader(tr, limit)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	// Хвост записи при declaredSize > maxBody не дочитываем: для несовпавших записей следующий tr.Next()
	// сбросит остаток; для найденного файла чтение одноразовое — сразу defer Close на ReadCloser от CopyFromContainer.
	return data, nil
}

func normTarEntryName(name string) string {
	n := strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	n = strings.TrimPrefix(n, "./")
	return path.Clean(n)
}

func isSafeTarEntryName(name string) bool {
	n := strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	if strings.HasPrefix(n, "/") {
		return false
	}
	n = strings.TrimPrefix(n, "./")
	if n == "" {
		return false
	}
	if path.IsAbs(n) {
		return false
	}
	for _, part := range strings.Split(n, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

func tarNameMatchesArtifact(headerName, wantSuffix string) bool {
	if !isSafeTarEntryName(headerName) {
		return false
	}
	n := normTarEntryName(headerName)
	want := strings.TrimPrefix(path.Clean("/"+wantSuffix), "/")
	if n == want {
		return true
	}
	baseWant := path.Base(want)
	// Архив docker cp одного файла часто содержит только basename (5.7).
	if !strings.Contains(n, "/") && n == baseWant {
		return true
	}
	return false
}

// mergeArtifactResultsIntoFinalStatus вписывает CodeResult и применяет политику «контракт vs Docker» (5.7).
// infraStrict передаётся снаружи под st.mu (не вызывать lifecycleInfraStrictLocked изнутри — вложенный лок).
func mergeArtifactResultsIntoFinalStatus(fs *SandboxStatus, st *instanceState, insp *types.ContainerJSON, out *artifactCollectionOutcome, infraStrict bool) {
	if fs == nil {
		return
	}
	if out == nil {
		return
	}
	fs.Result = out.buildCodeResult()

	if infraStrict || (insp != nil && insp.State != nil && insp.State.OOMKilled) {
		// OOM / таймаут / стоп — SandboxStatus.Status первичен; JSON success не апгрейдит completed (5.7).
		return
	}
	if fs.Status == SandboxStatusCompleted && !out.StatusJSONOK {
		// Docker exit 0, но нет валидного контрактного status.json — не считаем задачу успешной (5.7).
		fs.Status = SandboxStatusFailed
	}
}

// collectArtifactsForRunner — обёртка над *client.Client (5.5 / 5.7).
func collectArtifactsForRunner(ctx context.Context, cli *client.Client, containerID string) (*artifactCollectionOutcome, error) {
	if cli == nil {
		return nil, fmt.Errorf("docker client is nil: %w", ErrSandboxDocker)
	}
	return collectSandboxArtifacts(ctx, cli, containerID)
}
