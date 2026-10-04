//go:build integration

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1 "github.com/inf0-dev/alma/api/v1"
	"github.com/inf0-dev/alma/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDocument() *v1.Document {
	return &v1.Document{
		Metadata: v1.Metadata{Version: v1.VersionV1},
		Schema: v1.Schema{
			Title: "Test Decision",
			Requirements: []v1.Requirement{
				{ID: "hard1", Description: "Hard req", IsHard: true},
				{ID: "soft1", Description: "Soft req", IsHard: false},
			},
			Items: []v1.Item{
				{
					ID: "q1", Kind: v1.KindChoice, Description: "Q1",
					ChoiceOptions: []v1.ChoiceOption{
						{ID: "yes", Description: "Yes"},
						{ID: "no", Description: "No"},
					},
				},
			},
			DesignOptions: []v1.DesignOption{
				{
					ID: "opt1", Title: "Option 1",
					RequirementsMet: v1.RequirementsMet{
						"hard1": {Met: true},
						"soft1": {Met: true},
					},
				},
				{
					ID: "opt2", Title: "Option 2",
					RequirementsMet: v1.RequirementsMet{
						"hard1": {Met: false},
						"soft1": {Met: true},
					},
				},
			},
		},
	}
}

func testRecord() *v1.Record {
	doc := testDocument()
	return &v1.Record{
		Version:     v1.RecordVersionV1,
		Model:       *doc,
		ModelSHA256: "testhash",
		Requirements: []v1.RecordRequirement{
			{ID: "hard1", IsHard: true, Checked: true},
			{ID: "soft1", IsHard: false, Checked: true},
		},
		Items: []v1.RecordItem{
			{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
		},
		Options: []v1.RecordOption{
			{ID: "opt1", Status: v1.StatusPossible},
			{ID: "opt2", Status: v1.StatusEliminated},
		},
	}
}

func startServer(t *testing.T, rec *v1.Record) (*httptest.Server, *Server) {
	t.Helper()
	s := New(Config{Addr: ":0"}, rec)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, s
}

func multipartBody(t *testing.T, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

func assertRecentTimestamp(t *testing.T, value string) {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err, "should be valid RFC3339")
	assert.WithinDuration(t, time.Now().UTC(), parsed, 5*time.Second)
}

func TestHealthz(t *testing.T) {
	tests := []struct {
		name         string
		shuttingDown bool
		wantCode     int
		wantBody     string
	}{
		{
			name:     "healthy",
			wantCode: http.StatusOK,
			wantBody: "ok",
		},
		{
			name:         "shutting down",
			shuttingDown: true,
			wantCode:     http.StatusServiceUnavailable,
			wantBody:     "shutting down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, s := startServer(t, nil)
			if tt.shuttingDown {
				s.shuttingDown.Store(true)
			}

			resp, err := http.Get(ts.URL + "/healthz")
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantCode, resp.StatusCode)

			var body bytes.Buffer
			_, err = body.ReadFrom(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBody, body.String())
		})
	}
}

func TestPage(t *testing.T) {
	tests := []struct {
		name     string
		record   *v1.Record
		path     string
		wantCode int
	}{
		{
			name:     "root without record",
			record:   nil,
			path:     "/",
			wantCode: http.StatusOK,
		},
		{
			name:     "root with record",
			record:   testRecord(),
			path:     "/",
			wantCode: http.StatusOK,
		},
		{
			name:     "non-root path returns 404",
			record:   testRecord(),
			path:     "/nonexistent",
			wantCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, _ := startServer(t, tt.record)

			resp, err := http.Get(ts.URL + tt.path)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantCode, resp.StatusCode)
			if tt.wantCode == http.StatusOK {
				assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
			}
		})
	}
}

func TestExport(t *testing.T) {
	tests := []struct {
		name            string
		record          *v1.Record
		method          string
		query           string
		wantCode        int
		wantContentType string
		wantDisposition string
	}{
		{
			name:     "no record loaded",
			record:   nil,
			method:   http.MethodGet,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "wrong method",
			record:   testRecord(),
			method:   http.MethodPost,
			wantCode: http.StatusMethodNotAllowed,
		},
		{
			name:            "default format is YAML",
			record:          testRecord(),
			method:          http.MethodGet,
			wantCode:        http.StatusOK,
			wantContentType: "yaml",
			wantDisposition: ".yaml",
		},
		{
			name:            "JSON format",
			record:          testRecord(),
			method:          http.MethodGet,
			query:           "format=json",
			wantCode:        http.StatusOK,
			wantContentType: "application/json",
			wantDisposition: ".json",
		},
		{
			name:            "Markdown format",
			record:          testRecord(),
			method:          http.MethodGet,
			query:           "format=md",
			wantCode:        http.StatusOK,
			wantContentType: "text/markdown",
			wantDisposition: ".md",
		},
		{
			name:     "unsupported format",
			record:   testRecord(),
			method:   http.MethodGet,
			query:    "format=xml",
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, _ := startServer(t, tt.record)

			url := ts.URL + "/export"
			if tt.query != "" {
				url += "?" + tt.query
			}

			req, err := http.NewRequest(tt.method, url, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantCode, resp.StatusCode)
			if tt.wantContentType != "" {
				assert.Contains(t, resp.Header.Get("Content-Type"), tt.wantContentType)
			}
			if tt.wantDisposition != "" {
				assert.Contains(t, resp.Header.Get("Content-Disposition"), tt.wantDisposition)
			}
		})
	}
}

func TestExportJSON_Roundtrips(t *testing.T) {
	ts, _ := startServer(t, testRecord())

	resp, err := http.Get(ts.URL + "/export?format=json")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var rec v1.Record
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rec))
	assert.Equal(t, "Test Decision", rec.Model.Schema.Title)
}

func TestExportSanitizesFilename(t *testing.T) {
	rec := testRecord()
	rec.Model.Schema.Title = "My Cool Decision!"
	ts, _ := startServer(t, rec)

	resp, err := http.Get(ts.URL + "/export?format=json")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Contains(t, resp.Header.Get("Content-Disposition"), "my-cool-decision")
}

func TestState(t *testing.T) {
	tests := []struct {
		name     string
		record   *v1.Record
		method   string
		body     string
		wantCode int
	}{
		{
			name:     "wrong method",
			record:   testRecord(),
			method:   http.MethodGet,
			wantCode: http.StatusMethodNotAllowed,
		},
		{
			name:     "no record loaded",
			record:   nil,
			method:   http.MethodPatch,
			body:     `{"requirements":[],"items":[]}`,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "invalid JSON",
			record:   testRecord(),
			method:   http.MethodPatch,
			body:     "{bad",
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, _ := startServer(t, tt.record)

			req, err := http.NewRequest(tt.method, ts.URL+"/state", strings.NewReader(tt.body))
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantCode, resp.StatusCode)
		})
	}
}

func TestState_InvalidJSON_ReturnsCurrentState(t *testing.T) {
	ts, _ := startServer(t, testRecord())

	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/state", strings.NewReader("{bad"))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp stateErrorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	assert.NotEmpty(t, errResp.Error)
	assert.NotEmpty(t, errResp.Requirements)
}

func TestState_UpdatesRecord(t *testing.T) {
	ts, s := startServer(t, testRecord())

	payload := statePayload{
		Requirements: []v1.RecordRequirement{
			{ID: "hard1", IsHard: true, Checked: true},
			{ID: "soft1", IsHard: false, Checked: false},
		},
		Items: []v1.RecordItem{
			{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("no")},
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/state", bytes.NewReader(body))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	s.mu.RLock()
	defer s.mu.RUnlock()
	assert.Equal(t, "no", *s.record.Items[0].Answer)
}

func TestState_PreservesOutcome(t *testing.T) {
	rec := testRecord()
	rec.Final = &v1.FinalDecision{Option: "opt1", Title: "Option 1", Rationale: "Best"}
	rec.Present = []string{"Alice"}
	rec.DecidedOn = "2026-10-01T14:30:00Z"
	rec.History = []v1.HistoryEntry{
		{DecidedOn: "2026-09-01", Option: "opt2", Title: "Option 2"},
	}
	ts, s := startServer(t, rec)

	payload := statePayload{
		Requirements: rec.Requirements,
		Items:        rec.Items,
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/state", bytes.NewReader(body))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	s.mu.RLock()
	defer s.mu.RUnlock()
	require.NotNil(t, s.record.Final)
	assert.Equal(t, "opt1", s.record.Final.Option)
	assert.Equal(t, []string{"Alice"}, s.record.Present)
	assert.Equal(t, "2026-10-01T14:30:00Z", s.record.DecidedOn)
	assert.Len(t, s.record.History, 1)
}

func TestFinalizePost(t *testing.T) {
	tests := []struct {
		name     string
		record   *v1.Record
		body     string
		wantCode int
	}{
		{
			name:     "no record loaded",
			record:   nil,
			body:     `{"option":"opt1"}`,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "invalid JSON",
			record:   testRecord(),
			body:     "{bad",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "unknown option ID",
			record:   testRecord(),
			body:     `{"option":"opt-nonexistent"}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "eliminated option rejected",
			record:   testRecord(),
			body:     `{"option":"opt2","rationale":"Trying eliminated"}`,
			wantCode: http.StatusConflict,
		},
		{
			name:     "picks possible option",
			record:   testRecord(),
			body:     `{"option":"opt1","rationale":"Best choice","present":["Alice","Bob"]}`,
			wantCode: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, _ := startServer(t, tt.record)

			resp, err := http.Post(ts.URL+"/finalize", "application/json", strings.NewReader(tt.body))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantCode, resp.StatusCode)
		})
	}
}

func TestFinalizePost_SetsFields(t *testing.T) {
	ts, s := startServer(t, testRecord())

	body := `{"option":"opt1","rationale":"Best choice","present":["Alice","Bob"]}`
	resp, err := http.Post(ts.URL+"/finalize", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	s.mu.RLock()
	defer s.mu.RUnlock()
	require.NotNil(t, s.record.Final)
	assert.Equal(t, "opt1", s.record.Final.Option)
	assert.Equal(t, "Option 1", s.record.Final.Title)
	assert.Equal(t, "Best choice", s.record.Final.Rationale)
	assert.Equal(t, []string{"Alice", "Bob"}, s.record.Present)
	assertRecentTimestamp(t, s.record.DecidedOn)
}

func TestFinalizePost_ReplacesExistingDecision(t *testing.T) {
	rec := testRecord()
	rec.Final = &v1.FinalDecision{Option: "opt1", Title: "Option 1", Rationale: "Old reason"}
	rec.Present = []string{"Charlie"}
	rec.DecidedOn = "2026-09-01T10:00:00Z"
	rec.ModelSHA256 = "hash123"
	ts, s := startServer(t, rec)

	body := `{"option":"opt1","rationale":"New reason","present":["Alice"]}`
	resp, err := http.Post(ts.URL+"/finalize", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	s.mu.RLock()
	defer s.mu.RUnlock()

	require.Len(t, s.record.History, 1)
	assert.Equal(t, "opt1", s.record.History[0].Option)
	assert.Equal(t, "Old reason", s.record.History[0].Rationale)
	assert.Equal(t, []string{"Charlie"}, s.record.History[0].Present)
	assert.Equal(t, "2026-09-01T10:00:00Z", s.record.History[0].DecidedOn)
	assert.Equal(t, "hash123", s.record.History[0].ModelSHA256)

	assert.Equal(t, "New reason", s.record.Final.Rationale)
	assert.Equal(t, []string{"Alice"}, s.record.Present)
}

func TestFinalizePost_AppendsToExistingHistory(t *testing.T) {
	rec := testRecord()
	rec.History = []v1.HistoryEntry{
		{DecidedOn: "2026-08-01", Option: "opt1", Title: "First"},
	}
	rec.Final = &v1.FinalDecision{Option: "opt1", Title: "Option 1", Rationale: "Second"}
	rec.DecidedOn = "2026-09-01T10:00:00Z"
	rec.Present = []string{"Bob"}
	ts, s := startServer(t, rec)

	body := `{"option":"opt1","rationale":"Third","present":["Alice"]}`
	resp, err := http.Post(ts.URL+"/finalize", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	s.mu.RLock()
	defer s.mu.RUnlock()
	require.Len(t, s.record.History, 2)
	assert.Equal(t, "First", s.record.History[0].Title)
	assert.Equal(t, "Second", s.record.History[1].Rationale)
}

func TestFinalizeDelete(t *testing.T) {
	tests := []struct {
		name        string
		record      *v1.Record
		wantCode    int
		wantHistory int
	}{
		{
			name:     "no record loaded",
			record:   nil,
			wantCode: http.StatusNotFound,
		},
		{
			name:        "no existing decision is a noop",
			record:      testRecord(),
			wantCode:    http.StatusNoContent,
			wantHistory: 0,
		},
		{
			name: "clears decision and moves to history",
			record: func() *v1.Record {
				rec := testRecord()
				rec.Final = &v1.FinalDecision{Option: "opt1", Title: "Option 1", Rationale: "Undoing"}
				rec.Present = []string{"Alice"}
				rec.DecidedOn = "2026-10-01T14:30:00Z"
				rec.ModelSHA256 = "hash456"
				return rec
			}(),
			wantCode:    http.StatusNoContent,
			wantHistory: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, s := startServer(t, tt.record)

			req, err := http.NewRequest(http.MethodDelete, ts.URL+"/finalize", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantCode, resp.StatusCode)

			if tt.record != nil {
				s.mu.RLock()
				defer s.mu.RUnlock()
				assert.Len(t, s.record.History, tt.wantHistory)
			}
		})
	}
}

func TestFinalizeDelete_ClearsFields(t *testing.T) {
	rec := testRecord()
	rec.Final = &v1.FinalDecision{Option: "opt1", Title: "Option 1", Rationale: "Undoing"}
	rec.Present = []string{"Alice"}
	rec.DecidedOn = "2026-10-01T14:30:00Z"
	rec.ModelSHA256 = "hash456"
	ts, s := startServer(t, rec)

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/finalize", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	s.mu.RLock()
	defer s.mu.RUnlock()
	assert.Nil(t, s.record.Final)
	assert.Nil(t, s.record.Present)
	assert.Empty(t, s.record.DecidedOn)

	require.Len(t, s.record.History, 1)
	assert.Equal(t, "opt1", s.record.History[0].Option)
	assert.Equal(t, "Undoing", s.record.History[0].Rationale)
	assert.Equal(t, "hash456", s.record.History[0].ModelSHA256)
}

func TestFinalize_WrongMethod(t *testing.T) {
	ts, _ := startServer(t, testRecord())

	resp, err := http.Get(ts.URL + "/finalize")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestUpload(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		body     func(t *testing.T) (*bytes.Buffer, string)
		wantCode int
	}{
		{
			name:   "wrong method",
			method: http.MethodGet,
			body: func(t *testing.T) (*bytes.Buffer, string) {
				return &bytes.Buffer{}, ""
			},
			wantCode: http.StatusMethodNotAllowed,
		},
		{
			name:   "missing file field",
			method: http.MethodPost,
			body: func(t *testing.T) (*bytes.Buffer, string) {
				return &bytes.Buffer{}, "multipart/form-data; boundary=x"
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name:   "invalid document content",
			method: http.MethodPost,
			body: func(t *testing.T) (*bytes.Buffer, string) {
				return multipartBody(t, "bad.yaml", []byte("not: valid: alma"))
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name:   "valid record",
			method: http.MethodPost,
			body: func(t *testing.T) (*bytes.Buffer, string) {
				return multipartBody(t, "record.yaml", []byte(validRecordYAML))
			},
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, _ := startServer(t, nil)
			buf, contentType := tt.body(t)

			req, err := http.NewRequest(tt.method, ts.URL+"/upload", buf)
			require.NoError(t, err)
			if contentType != "" {
				req.Header.Set("Content-Type", contentType)
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantCode, resp.StatusCode)
		})
	}
}

func TestUpload_LoadsRecord(t *testing.T) {
	ts, s := startServer(t, nil)
	buf, contentType := multipartBody(t, "record.yaml", []byte(validRecordYAML))

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/upload", buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	s.mu.RLock()
	defer s.mu.RUnlock()
	require.NotNil(t, s.record)
	assert.Equal(t, "Cache Strategy Decision", s.record.Model.Schema.Title)
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Test Decision", "test-decision"},
		{"My Cool Decision!", "my-cool-decision"},
		{"  spaces  ", "spaces"},
		{"UPPER_case", "upper-case"},
		{"", "record"},
		{"!!!??", "record"},
		{"a-b-c", "a-b-c"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			assert.Equal(t, tt.want, slugify(tt.input))
		})
	}
}

func TestCSPHeader(t *testing.T) {
	tests := []struct {
		name         string
		wantContains string
	}{
		{"default-src", "default-src 'none'"},
		{"script-src", "script-src 'unsafe-inline'"},
		{"style-src", "style-src"},
		{"connect-src", "connect-src 'self'"},
	}

	ts, _ := startServer(t, nil)

	resp, err := http.Get(ts.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	csp := resp.Header.Get("Content-Security-Policy")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, csp, tt.wantContains)
		})
	}
}

func TestFullWorkflow_UploadThenFinalizeThenExport(t *testing.T) {
	ts, _ := startServer(t, nil)
	client := ts.Client()

	// upload a record
	buf, contentType := multipartBody(t, "record.yaml", []byte(validRecordYAML))
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/upload", buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)

	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// page should render
	resp, err = client.Get(ts.URL + "/")
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// finalize decision
	resp, err = client.Post(
		ts.URL+"/finalize",
		"application/json",
		strings.NewReader(`{"option":"opt-redis","rationale":"Best fit","present":["Alice"]}`),
	)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// export as JSON
	resp, err = client.Get(ts.URL + "/export?format=json")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var rec v1.Record
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rec))
	require.NotNil(t, rec.Final)
	assert.Equal(t, "opt-redis", rec.Final.Option)
	assert.Equal(t, "Best fit", rec.Final.Rationale)
	assert.Equal(t, []string{"Alice"}, rec.Present)
}

const validRecordYAML = `version: "alma/record/v1"
model:
  metadata:
    version: "alma/v1"
    author: "Alice"
    date: "2026-09-15"
  schema:
    title: "Cache Strategy Decision"
    categories:
      - performance
    requirements:
      - id: req-latency
        description: "P99 latency under 50ms"
        is_hard: true
    items:
      - id: q-traffic
        description: "Expected peak traffic"
        kind: choice
        choice_options:
          - id: low
            description: "Under 1k RPS"
          - id: high
            description: "Over 10k RPS"
    design_options:
      - id: opt-redis
        title: "Redis Cluster"
        requirements_met:
          req-latency:
            met: true
model_sha256: "abc123"
decided_on: "2026-10-01T14:30:00Z"
requirements:
  - id: req-latency
    is_hard: true
    checked: true
items:
  - id: q-traffic
    kind: choice
    answer: high
options:
  - id: opt-redis
    status: possible
    meets:
      hard:
        met: 1
        of: 1
      soft:
        met: 0
        of: 0
    failed_requirements: []
    fired_blocks: []
still_open: []
`
