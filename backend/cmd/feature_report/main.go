// Command feature_report — генератор дашборда статусов фич по результатам
// `go test -json` (backend smoke) и `flutter test --machine` (frontend
// integration). Замысел — Task 5.3 в docs/integration-tests-plan.md.
//
// Назначение: после CI-прогона сложить матрицу «фича X: ✅/❌» в виде:
//   - Markdown (для комментария к PR / GitHub Pages),
//   - HTML (для GitHub Pages — единая страница с матрицей).
//
// Контракт ввода:
//
//	--backend  path/to/go-test.json           # output of `go test -json`
//	--frontend path/to/flutter-machine.json   # output of `flutter test --machine`
//	--out-md   path/to/report.md
//	--out-html path/to/report.html
//	--title    "Feature smoke (PR-gate)"
//	--commit   <git sha>                      # подсветка в шапке отчёта
//
// Обязателен только хотя бы один из --backend/--frontend. Остальные — опциональны:
// если флаг не передан, соответствующая секция в отчёте просто помечается «не запускалось».
//
// Маппинг тестов → «фичи»:
//
//	Backend: `TestFoo_Bar` (pkg .../test/featuresmoke) →
//	         feature = «Foo» (всё до первого «_»). Это совпадает с принятой
//	         схемой именования: TestAuth_*, TestTasks_*, TestProjects_*, и т.д.
//
//	Frontend: путь до test-файла → feature = `<flow>_test.dart` без суффикса.
//	          Например: `auth_flow_test.dart` → «auth_flow».
//
// Для каждой фичи: если ВСЕ тесты прошли — ✅; если хотя бы один зафейлился — ❌;
// если все skip'ed — ⚪ (нейтрально, отдельная колонка «skipped»). Это даёт
// именно «матрицу», которую видит ревьювер: одна строка = одна фича.
//
// Запуск:
//
//	go run ./backend/cmd/feature_report \
//	    --backend ./artifacts/backend.json \
//	    --frontend ./artifacts/frontend.json \
//	    --out-md ./artifacts/report.md \
//	    --out-html ./artifacts/report.html
//
// Без зависимостей: только stdlib (encoding/json, html/template). Это
// сознательное решение — генератор должен прогоняться в CI без go.mod download'а
// сверх того, что уже есть в backend/.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// goTestEvent — событие из `go test -json`. Соответствует
// https://pkg.go.dev/cmd/test2json.
type goTestEvent struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`  // run/pass/fail/skip/output/...
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Elapsed float64   `json:"Elapsed"`
	Output  string    `json:"Output"`
}

// flutterEvent — событие из `flutter test --machine` (упрощённое). Спецификация:
// https://github.com/flutter/flutter/blob/master/docs/contributing/testing/Test-suite-output-format.md
//
// Нас интересуют:
//
//	type=testStart  → id, name, suiteID, line, column, url=path/to/file
//	type=testDone   → testID, result (success/failure/error), hidden, skipped
//	type=suite      → suite.id, suite.path  (мы достаём путь файла по suiteID)
type flutterEvent struct {
	Type    string `json:"type"`
	Suite   *struct {
		ID       int    `json:"id"`
		Platform string `json:"platform"`
		Path     string `json:"path"`
	} `json:"suite,omitempty"`
	Test *struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		SuiteID int    `json:"suiteID"`
		URL     string `json:"url"`
	} `json:"test,omitempty"`
	TestID  int    `json:"testID,omitempty"`
	Result  string `json:"result,omitempty"`
	Hidden  bool   `json:"hidden,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
}

// featureStatus — агрегат по одной фиче.
type featureStatus struct {
	Name    string
	Source  string // "backend" | "frontend"
	Total   int
	Passed  int
	Failed  int
	Skipped int
	Tests   []testResult // отсортированы по имени
}

type testResult struct {
	Name    string
	Status  string // "pass" | "fail" | "skip"
	Elapsed float64
}

// emoji возвращает символ для status фичи. Никаких реальных эмодзи в Go-коде
// (мы их пишем только если пользователь явно об этом просит); для отчёта
// используем символьные glyphs, доступные в любом UTF-8 рендерере.
func (f featureStatus) Status() string {
	switch {
	case f.Failed > 0:
		return "FAIL"
	case f.Passed > 0:
		return "PASS"
	case f.Skipped > 0:
		return "SKIP"
	default:
		return "—"
	}
}

// CssClass — для HTML-таблицы.
func (f featureStatus) CssClass() string {
	switch f.Status() {
	case "FAIL":
		return "row-fail"
	case "PASS":
		return "row-pass"
	case "SKIP":
		return "row-skip"
	default:
		return ""
	}
}

func main() {
	var (
		backendPath  = flag.String("backend", "", "path to `go test -json` output")
		frontendPath = flag.String("frontend", "", "path to `flutter test --machine` output")
		outMD        = flag.String("out-md", "", "write Markdown report here")
		outHTML      = flag.String("out-html", "", "write HTML report here")
		title        = flag.String("title", "Feature smoke matrix", "report title")
		commit       = flag.String("commit", "", "commit sha to show in header")
		failOnRed    = flag.Bool("fail-on-red", false, "exit 1 if any feature is red (CI gate)")
	)
	flag.Parse()

	if *backendPath == "" && *frontendPath == "" {
		fmt.Fprintln(os.Stderr, "feature_report: at least one of --backend/--frontend is required")
		os.Exit(2)
	}
	if *outMD == "" && *outHTML == "" {
		fmt.Fprintln(os.Stderr, "feature_report: at least one of --out-md/--out-html is required")
		os.Exit(2)
	}

	var backendFeatures, frontendFeatures []featureStatus
	if *backendPath != "" {
		evs, err := parseGoTestJSON(*backendPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "feature_report: parse backend JSON %s: %v\n", *backendPath, err)
			os.Exit(2)
		}
		backendFeatures = aggregateBackend(evs)
	}
	if *frontendPath != "" {
		evs, err := parseFlutterJSON(*frontendPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "feature_report: parse frontend JSON %s: %v\n", *frontendPath, err)
			os.Exit(2)
		}
		frontendFeatures = aggregateFrontend(evs)
	}

	report := buildReport(*title, *commit, backendFeatures, frontendFeatures)

	if *outMD != "" {
		if err := os.WriteFile(*outMD, []byte(renderMarkdown(report)), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "feature_report: write %s: %v\n", *outMD, err)
			os.Exit(2)
		}
	}
	if *outHTML != "" {
		raw, err := renderHTML(report)
		if err != nil {
			fmt.Fprintf(os.Stderr, "feature_report: render HTML: %v\n", err)
			os.Exit(2)
		}
		if err := os.WriteFile(*outHTML, []byte(raw), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "feature_report: write %s: %v\n", *outHTML, err)
			os.Exit(2)
		}
	}

	if *failOnRed && reportHasRed(report) {
		// Используем 1 (а не 2) — 2 зарезервирован за «cli usage error».
		os.Exit(1)
	}
}

// ─── parsing ────────────────────────────────────────────────────────────────

func parseGoTestJSON(path string) ([]goTestEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseGoTestStream(f)
}

func parseGoTestStream(r io.Reader) ([]goTestEvent, error) {
	out := make([]goTestEvent, 0, 1024)
	scan := bufio.NewScanner(r)
	// Каждое событие — одна строка. Output-события могут быть длинными
	// (трейсбэк), поэтому буфер увеличиваем до 1 МБ.
	scan.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scan.Scan() {
		line := scan.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev goTestEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			// Пропускаем нештатные строки (например, summary "FAIL" перед exit code).
			// `go test -json` сам по себе строго JSON-only, но make-обёртки иногда
			// смешивают stderr с stdout, и тут лучше быть мягким.
			continue
		}
		out = append(out, ev)
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parseFlutterJSON(path string) ([]flutterEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseFlutterStream(f)
}

func parseFlutterStream(r io.Reader) ([]flutterEvent, error) {
	out := make([]flutterEvent, 0, 1024)
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scan.Scan() {
		line := scan.Bytes()
		// Flutter machine-output иногда пишет JSON-массив на одной строке как
		// «[{...},{...}]» и иногда — по событию на строку. Поддержим оба варианта.
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			var arr []flutterEvent
			if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
				out = append(out, arr...)
				continue
			}
		}
		var ev flutterEvent
		if err := json.Unmarshal([]byte(trimmed), &ev); err != nil {
			continue
		}
		out = append(out, ev)
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ─── aggregation ────────────────────────────────────────────────────────────

// aggregateBackend группирует go-test'ы по «фиче» = первая часть имени до «_».
// Тесты без `_` в имени мапятся на feature = весь test name.
func aggregateBackend(evs []goTestEvent) []featureStatus {
	type key struct{ feature, test string }
	state := map[key]string{} // последний action для test
	elapsed := map[key]float64{}
	for _, ev := range evs {
		if ev.Test == "" {
			continue
		}
		// Поддерживаем sub-test'ы: для отчёта работаем на верхнем уровне
		// (TestFoo_Bar/sub → TestFoo_Bar). Это даёт стабильную аггрегацию
		// независимо от количества sub-test'ов.
		topTest := ev.Test
		if i := strings.Index(topTest, "/"); i >= 0 {
			topTest = topTest[:i]
		}
		feature := featureNameFromGoTest(topTest)
		k := key{feature, topTest}
		switch ev.Action {
		case "pass", "fail", "skip":
			state[k] = ev.Action
			if ev.Elapsed > 0 {
				elapsed[k] = ev.Elapsed
			}
		}
	}
	// Сгруппировать в featureStatus.
	byFeat := map[string]*featureStatus{}
	for k, status := range state {
		fs, ok := byFeat[k.feature]
		if !ok {
			fs = &featureStatus{Name: k.feature, Source: "backend"}
			byFeat[k.feature] = fs
		}
		fs.Total++
		switch status {
		case "pass":
			fs.Passed++
		case "fail":
			fs.Failed++
		case "skip":
			fs.Skipped++
		}
		fs.Tests = append(fs.Tests, testResult{
			Name:    k.test,
			Status:  status,
			Elapsed: elapsed[k],
		})
	}
	return finalizeFeatures(byFeat)
}

// featureNameFromGoTest: `TestAuth_Register` → `Auth`. `TestE2EReal_Mixed…` → `E2EReal`.
// Если имя не начинается с `Test` (что для go-test'ов нештатно) — возвращаем имя как есть.
//
// Краевой случай: `Test_Helper` (legacy-naming, бывает) → TrimPrefix даст `_Helper`,
// дальше strings.Index найдёт `_` на индексе 0 и вернёт пустую строку. Тогда
// в матрице появится строка-фантом без имени. TrimLeft защищает от этого.
func featureNameFromGoTest(test string) string {
	name := strings.TrimPrefix(test, "Test")
	name = strings.TrimLeft(name, "_")
	if i := strings.Index(name, "_"); i >= 0 {
		return name[:i]
	}
	if name == "" {
		return test
	}
	return name
}

func aggregateFrontend(evs []flutterEvent) []featureStatus {
	// Стадия 1: запомнить пути файлов по suiteID.
	suitePath := map[int]string{}
	// Стадия 2: запомнить URL/имя теста по testID; testDone несёт только testID+result.
	type testInfo struct {
		Name    string
		SuiteID int
	}
	testMeta := map[int]testInfo{}
	type doneEv struct {
		ID      int
		Result  string
		Hidden  bool
		Skipped bool
	}
	doneEvs := make([]doneEv, 0, len(evs))
	for _, ev := range evs {
		switch ev.Type {
		case "suite":
			if ev.Suite != nil {
				suitePath[ev.Suite.ID] = ev.Suite.Path
			}
		case "testStart":
			if ev.Test != nil {
				testMeta[ev.Test.ID] = testInfo{Name: ev.Test.Name, SuiteID: ev.Test.SuiteID}
			}
		case "testDone":
			doneEvs = append(doneEvs, doneEv{
				ID: ev.TestID, Result: ev.Result, Hidden: ev.Hidden, Skipped: ev.Skipped,
			})
		}
	}

	byFeat := map[string]*featureStatus{}
	for _, d := range doneEvs {
		if d.Hidden {
			// Flutter резервирует hidden=true для технических wrapper'ов вроде «loading test_main».
			// Их пропускаем — иначе матрица засорится.
			continue
		}
		info := testMeta[d.ID]
		feature := featureNameFromFlutterPath(suitePath[info.SuiteID])
		if feature == "" {
			continue
		}
		fs, ok := byFeat[feature]
		if !ok {
			fs = &featureStatus{Name: feature, Source: "frontend"}
			byFeat[feature] = fs
		}
		fs.Total++
		var status string
		switch {
		case d.Skipped:
			fs.Skipped++
			status = "skip"
		case d.Result == "success":
			fs.Passed++
			status = "pass"
		default:
			fs.Failed++
			status = "fail"
		}
		fs.Tests = append(fs.Tests, testResult{Name: info.Name, Status: status})
	}
	return finalizeFeatures(byFeat)
}

func featureNameFromFlutterPath(p string) string {
	if p == "" {
		return ""
	}
	base := filepath.Base(p)
	base = strings.TrimSuffix(base, ".dart")
	base = strings.TrimSuffix(base, "_test")
	return base
}

func finalizeFeatures(byFeat map[string]*featureStatus) []featureStatus {
	out := make([]featureStatus, 0, len(byFeat))
	for _, fs := range byFeat {
		sort.Slice(fs.Tests, func(i, j int) bool { return fs.Tests[i].Name < fs.Tests[j].Name })
		out = append(out, *fs)
	}
	// Red-first порядок: легче ловить регрессии в верхней части матрицы.
	sort.Slice(out, func(i, j int) bool {
		// fail сверху, потом pass, потом skip.
		rank := func(s featureStatus) int {
			switch s.Status() {
			case "FAIL":
				return 0
			case "PASS":
				return 1
			case "SKIP":
				return 2
			default:
				return 3
			}
		}
		ri, rj := rank(out[i]), rank(out[j])
		if ri != rj {
			return ri < rj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// ─── rendering ──────────────────────────────────────────────────────────────

type reportSection struct {
	Heading  string
	Features []featureStatus
}

func (rs reportSection) HasFailures() bool {
	for _, f := range rs.Features {
		if f.Failed > 0 {
			return true
		}
	}
	return false
}

type report struct {
	Title    string
	Commit   string
	When     string
	Sections []reportSection
}

func buildReport(title, commit string, backend, frontend []featureStatus) report {
	return report{
		Title:  title,
		Commit: commit,
		When:   time.Now().UTC().Format(time.RFC3339),
		Sections: []reportSection{
			{Heading: "Backend smoke", Features: backend},
			{Heading: "Frontend integration", Features: frontend},
		},
	}
}

func reportHasRed(r report) bool {
	for _, sec := range r.Sections {
		for _, f := range sec.Features {
			if f.Status() == "FAIL" {
				return true
			}
		}
	}
	return false
}

// renderMarkdown — для PR-комментария. Без всяких HTML/CSS — это будет
// отрендерено GitHub'ом.
func renderMarkdown(r report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", r.Title)
	if r.Commit != "" {
		fmt.Fprintf(&b, "Commit: `%s`  \n", r.Commit)
	}
	fmt.Fprintf(&b, "Generated: %s\n\n", r.When)

	for _, sec := range r.Sections {
		fmt.Fprintf(&b, "## %s\n\n", sec.Heading)
		if len(sec.Features) == 0 {
			b.WriteString("_не запускалось_\n\n")
			continue
		}
		b.WriteString("| Feature | Status | Pass | Fail | Skip |\n")
		b.WriteString("|---|---|---:|---:|---:|\n")
		for _, f := range sec.Features {
			fmt.Fprintf(&b, "| `%s` | %s | %d | %d | %d |\n",
				escapeMD(f.Name), statusBadge(f.Status()),
				f.Passed, f.Failed, f.Skipped)
		}
		b.WriteString("\n")

		// Список упавших тестов — оператор сразу видит «что чинить».
		var failed []string
		for _, f := range sec.Features {
			for _, tr := range f.Tests {
				if tr.Status == "fail" {
					failed = append(failed, "`"+f.Name+"` :: `"+tr.Name+"`")
				}
			}
		}
		if len(failed) > 0 {
			b.WriteString("<details><summary>Failed tests</summary>\n\n")
			for _, line := range failed {
				fmt.Fprintf(&b, "- %s\n", line)
			}
			b.WriteString("\n</details>\n\n")
		}
	}
	return b.String()
}

func statusBadge(s string) string {
	// GitHub Markdown поддерживает unicode прямо в таблицах. Используем
	// круглые маркеры (нет проблем с шрифтами в любом терминале).
	switch s {
	case "PASS":
		return "PASS (green)"
	case "FAIL":
		return "FAIL (red)"
	case "SKIP":
		return "SKIP (grey)"
	default:
		return s
	}
}

func escapeMD(s string) string {
	// Минимально безопасное экранирование для содержимого внутри `…`.
	// Имена тестов в Go — буквы/цифры/`_`/`/`, в Flutter — обычные строки;
	// «`» внутри теоретически возможен, и тогда таблица сломается.
	return strings.ReplaceAll(s, "`", "\\`")
}

// renderHTML — для GitHub Pages. html/template само экранирует.
func renderHTML(r report) (string, error) {
	tpl := template.Must(template.New("report").Funcs(template.FuncMap{
		"badge": statusBadge,
	}).Parse(htmlTemplate))
	var b strings.Builder
	if err := tpl.Execute(&b, r); err != nil {
		return "", err
	}
	return b.String(), nil
}

const htmlTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; max-width: 1080px; margin: 2rem auto; padding: 0 1rem; color: #1f2328; }
  h1 { border-bottom: 1px solid #d0d7de; padding-bottom: .3em; }
  h2 { margin-top: 2rem; }
  table { border-collapse: collapse; width: 100%; margin-bottom: 1rem; }
  th, td { border: 1px solid #d0d7de; padding: 6px 12px; text-align: left; }
  th { background: #f6f8fa; }
  td.num { text-align: right; font-variant-numeric: tabular-nums; }
  tr.row-fail { background: #ffeef0; }
  tr.row-pass { background: #f0fff4; }
  tr.row-skip { background: #f6f8fa; color: #57606a; }
  .meta { color: #57606a; font-size: 0.9em; }
  .pill { display: inline-block; padding: 2px 8px; border-radius: 999px; font-weight: 600; font-size: 0.85em; }
  .pill-pass { background: #1f883d; color: white; }
  .pill-fail { background: #cf222e; color: white; }
  .pill-skip { background: #6e7781; color: white; }
  details { margin-top: .5rem; }
  details > summary { cursor: pointer; color: #0969da; }
  ul.tests { font-family: ui-monospace, SFMono-Regular, monospace; font-size: 0.85em; }
</style>
</head>
<body>
<h1>{{.Title}}</h1>
<p class="meta">
  {{if .Commit}}Commit: <code>{{.Commit}}</code> · {{end}}
  Generated: {{.When}}
</p>

{{range .Sections}}
  <h2>{{.Heading}}</h2>
  {{if .Features}}
    <table>
      <thead><tr><th>Feature</th><th>Status</th><th class="num">Pass</th><th class="num">Fail</th><th class="num">Skip</th></tr></thead>
      <tbody>
      {{range .Features}}
        <tr class="{{.CssClass}}">
          <td><code>{{.Name}}</code></td>
          <td>
            {{if eq .Status "PASS"}}<span class="pill pill-pass">PASS</span>{{end}}
            {{if eq .Status "FAIL"}}<span class="pill pill-fail">FAIL</span>{{end}}
            {{if eq .Status "SKIP"}}<span class="pill pill-skip">SKIP</span>{{end}}
          </td>
          <td class="num">{{.Passed}}</td>
          <td class="num">{{.Failed}}</td>
          <td class="num">{{.Skipped}}</td>
        </tr>
      {{end}}
      </tbody>
    </table>
    {{if .HasFailures}}
      <details>
        <summary>Failed tests</summary>
        <ul class="tests">
        {{range $feat := .Features}}{{range $feat.Tests}}{{if eq .Status "fail"}}
          <li>{{$feat.Name}} :: {{.Name}}</li>
        {{end}}{{end}}{{end}}
        </ul>
      </details>
    {{end}}
  {{else}}
    <p class="meta"><em>не запускалось</em></p>
  {{end}}
{{end}}

</body>
</html>
`
