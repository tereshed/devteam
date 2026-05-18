package main

import (
	"strings"
	"testing"
)

// Sanity-проверка парсинга go-test JSON: 3 теста в 2 фичах, один fail, один
// skip — оба должны корректно агрегироваться, фичи отсортированы red-first.
func TestAggregateBackend_RedFirst(t *testing.T) {
	input := strings.Join([]string{
		`{"Action":"run","Test":"TestAuth_Register"}`,
		`{"Action":"pass","Test":"TestAuth_Register","Elapsed":0.12}`,
		`{"Action":"run","Test":"TestTasks_Lifecycle"}`,
		`{"Action":"fail","Test":"TestTasks_Lifecycle","Elapsed":0.31}`,
		`{"Action":"run","Test":"TestTasks_Cancel"}`,
		`{"Action":"skip","Test":"TestTasks_Cancel"}`,
	}, "\n")

	evs, err := parseGoTestStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	feats := aggregateBackend(evs)
	if len(feats) != 2 {
		t.Fatalf("ожидали 2 фичи, получили %d: %+v", len(feats), feats)
	}
	// Red-first: Tasks (FAIL) идёт раньше Auth (PASS).
	if feats[0].Name != "Tasks" {
		t.Fatalf("ожидали Tasks первой (FAIL), получили %s", feats[0].Name)
	}
	if feats[0].Status() != "FAIL" {
		t.Fatalf("ожидали FAIL для Tasks, получили %s", feats[0].Status())
	}
	if feats[0].Failed != 1 || feats[0].Skipped != 1 || feats[0].Passed != 0 {
		t.Fatalf("Tasks: ожидали 0/1/1, получили %d/%d/%d", feats[0].Passed, feats[0].Failed, feats[0].Skipped)
	}
	if feats[1].Name != "Auth" || feats[1].Status() != "PASS" {
		t.Fatalf("ожидали Auth/PASS вторым, получили %s/%s", feats[1].Name, feats[1].Status())
	}
}

// Под Flutter machine-format: testStart → testDone, фича = basename файла без _test.dart.
func TestAggregateFrontend_FeatureFromPath(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"suite","suite":{"id":1,"platform":"chrome","path":"/abs/path/auth_flow_test.dart"}}`,
		`{"type":"testStart","test":{"id":10,"name":"login redirects","suiteID":1}}`,
		`{"type":"testDone","testID":10,"result":"success"}`,
		`{"type":"testStart","test":{"id":11,"name":"hidden","suiteID":1}}`,
		`{"type":"testDone","testID":11,"result":"success","hidden":true}`,
		`{"type":"suite","suite":{"id":2,"platform":"chrome","path":"/abs/path/projects_flow_test.dart"}}`,
		`{"type":"testStart","test":{"id":20,"name":"create project","suiteID":2}}`,
		`{"type":"testDone","testID":20,"result":"failure"}`,
	}, "\n")

	evs, err := parseFlutterStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	feats := aggregateFrontend(evs)
	if len(feats) != 2 {
		t.Fatalf("ожидали 2 фичи (hidden пропускается), получили %d: %+v", len(feats), feats)
	}
	// Red-first: projects_flow первой.
	if feats[0].Name != "projects_flow" || feats[0].Failed != 1 {
		t.Fatalf("ожидали projects_flow/FAIL первой, получили %+v", feats[0])
	}
	if feats[1].Name != "auth_flow" || feats[1].Passed != 1 {
		t.Fatalf("ожидали auth_flow/PASS второй, получили %+v", feats[1])
	}
}

// Rendering: Markdown содержит обе секции и упавшие тесты в details.
func TestRenderMarkdown_ContainsSections(t *testing.T) {
	r := buildReport("Feature smoke", "abc123",
		[]featureStatus{{Name: "Auth", Passed: 1, Total: 1,
			Tests: []testResult{{Name: "TestAuth_Register", Status: "pass"}}}},
		[]featureStatus{{Name: "auth_flow", Failed: 1, Total: 1,
			Tests: []testResult{{Name: "login redirects", Status: "fail"}}}},
	)
	md := renderMarkdown(r)
	for _, want := range []string{
		"# Feature smoke",
		"Commit: `abc123`",
		"## Backend smoke",
		"## Frontend integration",
		"| `Auth` | PASS",
		"| `auth_flow` | FAIL",
		"Failed tests",
		"login redirects",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown не содержит %q\n%s", want, md)
		}
	}
}

func TestRenderHTML_NoTemplateErrors(t *testing.T) {
	r := buildReport("Feature smoke", "abc123",
		[]featureStatus{{Name: "Auth", Passed: 1, Total: 1,
			Tests: []testResult{{Name: "TestAuth_Register", Status: "pass"}}}},
		nil,
	)
	html, err := renderHTML(r)
	if err != nil {
		t.Fatalf("renderHTML: %v", err)
	}
	for _, want := range []string{
		"<title>Feature smoke</title>",
		"Backend smoke",
		"<code>Auth</code>",
		"pill-pass",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("html не содержит %q\n%s", want, html)
		}
	}
}

// Краевой случай: имена тестов с leading underscore (`Test_Helper`) не должны
// давать пустую фичу — иначе в матрице появятся row-фантомы.
func TestFeatureNameFromGoTest_LeadingUnderscore(t *testing.T) {
	cases := map[string]string{
		"TestAuth_Register":           "Auth",
		"TestE2EReal_MixedPipeline":   "E2EReal",
		"Test_Helper":                 "Helper",                 // legacy edge case
		"Test__DoubleUnderscore":      "DoubleUnderscore",       // защита от любого числа `_`
		"TestNoUnderscores":           "NoUnderscores",          // без `_` после Test
		"TestFoo":                     "Foo",
	}
	for input, want := range cases {
		got := featureNameFromGoTest(input)
		if got != want {
			t.Errorf("featureNameFromGoTest(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestReportHasRed(t *testing.T) {
	clean := buildReport("t", "", []featureStatus{{Name: "x", Passed: 1, Total: 1}}, nil)
	if reportHasRed(clean) {
		t.Fatal("clean report не должен быть red")
	}
	red := buildReport("t", "", []featureStatus{{Name: "x", Failed: 1, Total: 1}}, nil)
	if !reportHasRed(red) {
		t.Fatal("red report должен быть red")
	}
}
