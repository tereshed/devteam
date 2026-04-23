package indexer

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/parser"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/vectordb"
	"github.com/google/uuid"
	"github.com/pkoukk/tiktoken-go"
	"log/slog"
)

const (
	MaxTokensPerChunk = 512
	ChunkOverlap      = 50
	BatchSize         = 50
	MaxBatchBytes     = 4 * 1024 * 1024 // 4MB
	MaxRecursionDepth = 10
	FileTimeout       = 10 * time.Second
	MaxLineLength     = 10240 // 10KB
	MaxErrorsPerProject = 100
	LargeFileThreshold  = 1024 * 1024 // 1MB
	MaxSearchQueryLen   = 4096
	MaxSearchLimit      = 50
	DefaultSearchLimit  = 10
	RelevanceThreshold  = 0.7 // Минимальная уверенность (certainty) для результатов
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(api[_-]?key|secret|password|token|auth|credential)\b\s*[:=]\s*['"][a-z0-9\-_]{8,128}['"]`),
	regexp.MustCompile(`(?i)\bbearer\b\s+[a-z0-9\-\._]{20,256}`),
	regexp.MustCompile(`\bghp_[a-zA-Z0-9]{36}\b`), // GitHub Personal Access Token
	regexp.MustCompile(`\bxox[baprs]-[a-zA-Z0-9-]{10,48}\b`), // Slack tokens
}

type codeIndexer struct {
	syncRepo    repository.SyncStateRepository
	vectorRepo  repository.VectorRepository
	parserPool  *sync.Pool
	numWorkers  int
	tokenizer   *tiktoken.Tiktoken
	logger      *slog.Logger
	errorCounts map[uuid.UUID]int
	errorMu     sync.Mutex
}

func NewCodeIndexer(
	syncRepo repository.SyncStateRepository,
	vectorRepo repository.VectorRepository,
	parserFactory func() parser.CodeParser,
	numWorkers int,
	logger *slog.Logger,
) (CodeIndexer, error) {
	if numWorkers <= 0 {
		numWorkers = 4
	}
	
	tkm, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding: %w", err)
	}

	return &codeIndexer{
		syncRepo:   syncRepo,
		vectorRepo: vectorRepo,
		parserPool: &sync.Pool{
			New: func() interface{} {
				return parserFactory()
			},
		},
		numWorkers:  numWorkers,
		tokenizer:   tkm,
		logger:      logger,
		errorCounts: make(map[uuid.UUID]int),
	}, nil
}

func (idx *codeIndexer) logError(projectID uuid.UUID, format string, args ...interface{}) {
	idx.errorMu.Lock()
	defer idx.errorMu.Unlock()

	count := idx.errorCounts[projectID]
	if count < MaxErrorsPerProject {
		fmt.Printf(format+"\n", args...)
		idx.errorCounts[projectID] = count + 1
	} else if count == MaxErrorsPerProject {
		fmt.Printf("Too many errors for project %s, suppressing further logs...\n", projectID)
		idx.errorCounts[projectID] = count + 1
	}
}

func (idx *codeIndexer) IndexProject(ctx context.Context, req IndexingRequest) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	filter := NewFileFilter(req.RepoPath)
	
	tasks := make(chan FileTask, idx.numWorkers*2)
	results := make(chan FileResult, idx.numWorkers*2)
	errChan := make(chan error, 1)

	var wg sync.WaitGroup

	// 1. Воркеры для обработки файлов
	for i := 0; i < idx.numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx.worker(ctx, tasks, results, errChan)
		}()
	}

	// 2. Сборщик результатов и запись в VectorDB
	done := make(chan struct{})
	processedFiles := make(map[string]string) // relPath -> hash
	var processedMu sync.Mutex

	go func() {
		defer close(done)
		idx.resultCollector(ctx, req.ProjectID, results, errChan, func(relPath, hash string) {
			processedMu.Lock()
			processedFiles[relPath] = hash
			processedMu.Unlock()
		})
		// Если resultCollector завершился (возможно с ошибкой), отменяем контекст для всех
		cancel()
	}()

	// 3. Сканирование файлов
	go func() {
		defer close(tasks)
		err := filepath.Walk(req.RepoPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			
			// Пропускаем .git и другие скрытые папки
			if info.IsDir() && (info.Name() == ".git" || info.Name() == "node_modules") {
				return filepath.SkipDir
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			should, err := filter.ShouldProcess(path)
			if err != nil {
				idx.logError(req.ProjectID, "Error filtering file %s: %v", path, err)
				return nil
			}
			if !should {
				return nil
			}

			relPath, err := filter.ValidatePath(path)
			if err != nil {
				return nil
			}

			// Проверяем, в процессе ли уже этот файл
			processedMu.Lock()
			isProcessed := false
			if _, ok := processedFiles[relPath]; ok {
				isProcessed = true
			}
			processedMu.Unlock()

			if isProcessed {
				return nil
			}

			select {
			case tasks <- FileTask{
				ProjectID:    req.ProjectID,
				RelativePath: relPath,
				AbsolutePath: path,
				Language:     filepath.Ext(path), // Передаем расширение, воркер определит язык
				Size:         info.Size(),
			}:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
		if err != nil {
			select {
			case errChan <- err:
				cancel()
			default:
			}
		}
	}()

	// Ожидание завершения обработки
	wg.Wait()
	close(results)
	<-done

	// 4. Cleanup: удаление из VectorDB файлов, которых больше нет в репозитории
	if err := idx.cleanupDeletedFiles(ctx, req.ProjectID, processedFiles); err != nil {
		idx.logError(req.ProjectID, "Error during cleanup: %v", err)
	}

	// Очистка счетчика ошибок после завершения проекта
	idx.errorMu.Lock()
	delete(idx.errorCounts, req.ProjectID)
	idx.errorMu.Unlock()

	// Проверка ошибок
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

func (idx *codeIndexer) cleanupDeletedFiles(ctx context.Context, projectID uuid.UUID, processedFiles map[string]string) error {
	states, err := idx.syncRepo.ListByProject(ctx, projectID)
	if err != nil {
		return err
	}

	for _, state := range states {
		if _, ok := processedFiles[state.FilePath]; !ok {
			// Файл удален
			if err := idx.vectorRepo.DeleteByContentID(ctx, projectID.String(), state.ID.String()); err != nil {
				idx.logError(projectID, "Error deleting from vector DB: %v", err)
			}
			if err := idx.syncRepo.Delete(ctx, projectID, state.FilePath); err != nil {
				idx.logError(projectID, "Error deleting from sync repo: %v", err)
			}
		}
	}
	return nil
}

func (idx *codeIndexer) worker(ctx context.Context, tasks <-chan FileTask, results chan<- FileResult, errChan chan<- error) {
	for task := range tasks {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Таймаут на обработку одного файла
		fileCtx, cancel := context.WithTimeout(ctx, FileTimeout)
		res, err := idx.processFile(fileCtx, task)
		cancel()

		if err != nil {
			idx.logError(task.ProjectID, "Error processing file %s: %v", task.RelativePath, err)
			continue
		}

		select {
		case results <- res:
		case <-ctx.Done():
			return
		}
	}
}

func (idx *codeIndexer) resultCollector(
	ctx context.Context,
	projectID uuid.UUID,
	results <-chan FileResult,
	errChan chan<- error,
	onFileProcessed func(relPath, hash string),
) {
	var batch []*models.VectorDocument
	var currentBatchBytes int
	
	// Список файлов, чьи чанки добавлены в текущий или прошлые батчи,
	// и которые готовы к обновлению SyncState после успешного flush.
	type pendingFile struct {
		relPath string
		hash    string
		fileID  uuid.UUID
	}
	var pendingFiles []pendingFile

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		_, err := idx.vectorRepo.BatchCreate(ctx, projectID.String(), batch)
		if err != nil {
			return err
		}

		// После успешной записи в VectorDB обновляем SyncState для всех накопленных файлов
		for _, f := range pendingFiles {
			err := idx.syncRepo.Upsert(ctx, &repository.FileSyncState{
				ID:          f.fileID,
				ProjectID:   projectID,
				FilePath:    f.relPath,
				ContentHash: f.hash,
				LastIndexed: time.Now().Unix(),
			})
			if err != nil {
				idx.logError(projectID, "Error updating sync state for %s: %v", f.relPath, err)
			}
			onFileProcessed(f.relPath, f.hash)
		}
		
		batch = nil
		currentBatchBytes = 0
		pendingFiles = nil
		return nil
	}

	for res := range results {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Если файл не изменился, просто помечаем его как обработанный
		if res.Unchanged {
			onFileProcessed(res.RelativePath, res.ContentHash)
			continue
		}

		if len(res.Chunks) == 0 {
			// Файл пустой или ошибка, которую мы пропустили, 
			// но нам нужно пометить его обработанным, чтобы не удалить
			onFileProcessed(res.RelativePath, res.ContentHash)
			continue
		}

		// Получаем существующее состояние или создаем новый ID
		state, _ := idx.syncRepo.GetByPath(ctx, projectID, res.RelativePath)
		var fileID uuid.UUID
		if state != nil {
			fileID = state.ID
			// Удаляем старые чанки перед обновлением.
			// Обязательно обрабатываем ошибку, чтобы избежать дубликатов.
			if err := idx.vectorRepo.DeleteByContentID(ctx, projectID.String(), fileID.String()); err != nil {
				idx.logError(projectID, "Failed to delete old chunks for %s: %v", res.RelativePath, err)
				continue // Пропускаем файл, чтобы не плодить дубликаты при сбое БД
			}
		} else {
			fileID = uuid.New()
		}

		for _, chunk := range res.Chunks {
			doc := models.NewVectorDocument(fileID.String(), chunk.Content, "code")
			doc.WithCategory("project_code")
			doc.SetMetadata("file_path", chunk.FilePath)
			doc.SetMetadata("language", chunk.Language)
			doc.SetMetadata("start_line", chunk.StartLine)
			doc.SetMetadata("end_line", chunk.EndLine)
			doc.SetMetadata("symbol", chunk.Symbol)
			doc.SetMetadata("project_id", projectID.String())
			doc.SetMetadata("content_hash", chunk.Hash)

			docBytes := len(chunk.Content) + len(chunk.FilePath) + 200

			if (len(batch) + 1) > BatchSize || (currentBatchBytes+docBytes) > MaxBatchBytes {
				if err := flush(); err != nil {
					select {
					case errChan <- err:
					default:
					}
					return
				}
			}

			batch = append(batch, doc)
			currentBatchBytes += docBytes
		}

		// Добавляем файл в список ожидающих обновления SyncState.
		// Его стейт обновится при следующем flush().
		pendingFiles = append(pendingFiles, pendingFile{
			relPath: res.RelativePath,
			hash:    res.ContentHash,
			fileID:  fileID,
		})
	}

	// Финальный flush для оставшихся чанков
	if err := flush(); err != nil {
		select {
		case errChan <- err:
		default:
		}
	}
}

func (idx *codeIndexer) maskSecrets(ctx context.Context, r io.Reader) (string, error) {
	var masked strings.Builder
	scanner := bufio.NewScanner(r)
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// Проверка длины строки (OOM protection)
		if len(line) > MaxLineLength {
			return "", fmt.Errorf("file contains abnormally long line (>%d bytes)", MaxLineLength)
		}

		if err := ctx.Err(); err != nil {
			// Если таймаут, возвращаем оставшуюся часть без изменений
			masked.WriteString(line)
			masked.WriteString("\n")
			continue
		}
		
		for _, pattern := range secretPatterns {
			line = pattern.ReplaceAllString(line, "[MASKED_SECRET]")
		}
		masked.WriteString(line)
		masked.WriteString("\n")
	}
	
	if err := scanner.Err(); err != nil {
		return "", err
	}
	
	return masked.String(), nil
}

func (idx *codeIndexer) processFile(ctx context.Context, task FileTask) (FileResult, error) {
	res := FileResult{
		ProjectID:    task.ProjectID,
		RelativePath: task.RelativePath,
	}

	file, err := os.Open(task.AbsolutePath)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			return res, nil
		}
		return res, err
	}
	defer file.Close()

	// Считаем хеш и проверяем SyncState в воркере
	// Используем LimitReader для защиты от OOM, если файл вырос после фильтрации
	hash := sha256.New()
	limitReader := io.LimitReader(file, MaxFileSize+1)
	written, err := io.Copy(hash, limitReader)
	if err != nil {
		return res, err
	}
	if written > MaxFileSize {
		return res, fmt.Errorf("file size exceeds limit during processing (TOCTOU)")
	}
	contentHash := hex.EncodeToString(hash.Sum(nil))
	res.ContentHash = contentHash

	// Проверяем SyncState
	if idx.syncRepo != nil {
		state, err := idx.syncRepo.GetByPath(ctx, task.ProjectID, task.RelativePath)
		if err == nil && state != nil && state.ContentHash == contentHash {
			res.Unchanged = true
			return res, nil // Файл не изменился
		}
	}

	// Возвращаемся в начало файла для маскировки и проверки длины строк
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return res, err
	}

	// 1. Маскировка секретов и проверка длины строк (в один проход)
	// Снова используем LimitReader для безопасности
	content, err := idx.maskSecrets(ctx, io.LimitReader(file, MaxFileSize))
	if err != nil {
		return res, err
	}

	// Получаем парсер из пула
	var p parser.CodeParser
	if idx.parserPool != nil {
		if poolVal := idx.parserPool.Get(); poolVal != nil {
			p = poolVal.(parser.CodeParser)
			defer func() {
				p.Reset()
				idx.parserPool.Put(p)
			}()
		}
	}

	lang := ""
	if p != nil {
		lang = p.GetLanguageByExtension(filepath.Ext(task.AbsolutePath))
	}

	var chunks []Chunk
	// 2. Попытка семантического разбиения через AST
	if p != nil {
		nodes, err := p.Parse(ctx, lang, []byte(content))
		if err == nil && len(nodes) > 0 {
			for _, node := range nodes {
				nodeChunks := idx.splitByTokens(ctx, p, node.Content, task.RelativePath, lang, node.StartLine, node.Symbol, 0)
				chunks = append(chunks, nodeChunks...)
			}
		} else {
			// 3. Fallback: разбиение по токенам
			chunks = idx.splitByTokens(ctx, p, content, task.RelativePath, lang, 1, "", 0)
		}
	} else {
		chunks = idx.splitByTokens(ctx, nil, content, task.RelativePath, lang, 1, "", 0)
	}

	for i := range chunks {
		chunks[i].FileHash = contentHash
	}

	res.Chunks = chunks
	return res, nil
}

func (idx *codeIndexer) splitByTokens(ctx context.Context, p parser.CodeParser, content, filePath, language string, startLine int, symbol string, depth int) []Chunk {
	tokens := idx.tokenizer.Encode(content, nil, nil)
	if len(tokens) <= MaxTokensPerChunk {
		hash := sha256.Sum256([]byte(content))
		chunkHash := hex.EncodeToString(hash[:])
		return []Chunk{{
			Content:   content,
			FilePath:  filePath,
			Language:  language,
			StartLine: startLine,
			EndLine:   startLine + strings.Count(content, "\n"),
			Symbol:    symbol,
			Hash:      chunkHash,
		}}
	}

	if depth < MaxRecursionDepth && p != nil {
		nodes, err := p.Parse(ctx, language, []byte(content))
		if err == nil && len(nodes) > 1 {
			var chunks []Chunk
			for _, node := range nodes {
				nodeChunks := idx.splitByTokens(ctx, p, node.Content, filePath, language, startLine+node.StartLine-1, node.Symbol, depth+1)
				chunks = append(chunks, nodeChunks...)
			}
			return chunks
		}
	}

	var chunks []Chunk
	currentStartLine := startLine
	
	for i := 0; i < len(tokens); i += (MaxTokensPerChunk - ChunkOverlap) {
		end := i + MaxTokensPerChunk
		if end > len(tokens) {
			end = len(tokens)
		}

		chunkTokens := tokens[i:end]
		chunkContent := idx.tokenizer.Decode(chunkTokens)
		
		numLines := strings.Count(chunkContent, "\n")
		
		hash := sha256.Sum256([]byte(chunkContent))
		chunkHash := hex.EncodeToString(hash[:])

		chunks = append(chunks, Chunk{
			Content:   chunkContent,
			FilePath:  filePath,
			Language:  language,
			StartLine: currentStartLine,
			EndLine:   currentStartLine + numLines,
			Symbol:    symbol,
			Hash:      chunkHash,
		})

		if end == len(tokens) {
			break
		}

		step := MaxTokensPerChunk - ChunkOverlap
		stepTokens := tokens[i : i+step]
		currentStartLine += strings.Count(idx.tokenizer.Decode(stepTokens), "\n")
	}

	return chunks
}

// mapSearchResultToChunk преобразует результат поиска из VectorDB в структуру Chunk
func mapSearchResultToChunk(res *vectordb.SearchResult) Chunk {
	chunk := Chunk{
		Content: res.Content,
	}

	if res.Metadata != nil {
		if val, ok := res.Metadata["file_path"].(string); ok {
			chunk.FilePath = val
		}
		if val, ok := res.Metadata["language"].(string); ok {
			chunk.Language = val
		}
		if val, ok := res.Metadata["symbol"].(string); ok {
			chunk.Symbol = val
		}
		if val, ok := res.Metadata["content_hash"].(string); ok {
			chunk.Hash = val
		}
		if startLine, ok := res.Metadata["start_line"].(float64); ok {
			chunk.StartLine = int(startLine)
		}
		if endLine, ok := res.Metadata["end_line"].(float64); ok {
			chunk.EndLine = int(endLine)
		}
	}

	return chunk
}

// SearchContext выполняет контекстный поиск по проиндексированному коду проекта
func (idx *codeIndexer) SearchContext(ctx context.Context, projectID uuid.UUID, query string, limit int) ([]Chunk, error) {
	// 1. Валидация
	if query == "" {
		return []Chunk{}, nil
	}
	if len(query) > MaxSearchQueryLen {
		return nil, ErrQueryTooLong
	}
	if limit <= 0 || limit > MaxSearchLimit {
		limit = DefaultSearchLimit
	}

	// 2. Поиск
	start := time.Now()

	// Используем гибридный поиск через VectorRepository
	// Мы хотим семантический поиск, поэтому Alpha ближе к 1.0, но оставим немного для ключевых слов
	params := &vectordb.SearchParams{
		ProjectID: projectID.String(),
		Query:     query,
		Category:  "project_code",
		Limit:     limit,
		Alpha:     0.7, // Смещение в сторону семантики
	}

	results, err := idx.vectorRepo.Search(ctx, projectID.String(), params)
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// 3. Фильтрация по релевантности и маппинг
	chunks := make([]Chunk, 0, len(results))
	for _, res := range results {
		// Weaviate distance: 0 - идентичны, 2 - противоположны (для косинусного расстояния)
		// Certainty обычно 1 - distance/2
		certainty := 1 - (res.Distance / 2)

		if certainty < RelevanceThreshold {
			continue
		}

		chunks = append(chunks, mapSearchResultToChunk(res))
	}

	// 4. Логирование
	if idx.logger != nil {
		idx.logger.Info("SearchContext completed",
			"project_id", projectID,
			"query_len", len(query),
			"limit", limit,
			"results_count", len(chunks),
			"total_found", len(results),
			"latency", duration.String(),
		)
	}

	return chunks, nil
}

// simpleTokenSplit больше не нужен

