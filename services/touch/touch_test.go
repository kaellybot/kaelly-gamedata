package touch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type request struct {
	method    string
	path      string
	query     string
	userAgent string
	body      map[string]string
}

func newTestService(t *testing.T, received *[]request) (*Impl, *httptest.Server) {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := request{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, userAgent: r.UserAgent()}
		_ = json.NewDecoder(r.Body).Decode(&req.body)
		*received = append(*received, req)

		switch r.URL.Path {
		case "/config.json":
			_, _ = w.Write([]byte(`{"dataUrl":"` + server.URL + `","assetsUrl":"https://cdn/assets/3.3.9_abc"}`))
		case buildScriptPath:
			_, _ = w.Write([]byte(`var a=1;window.buildVersion="1.74.8",window.appInfo={};`))
		case dataMapPath:
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(`{"26":{"id":26,"nameId":"Alchimiste"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	return &Impl{
		configURL: server.URL + "/config.json",
		userAgent: "KaellyBot-Gamedata/test",
		client:    server.Client(),
	}, server
}

func TestGetConfigAndBuildVersion(t *testing.T) {
	var received []request
	service, server := newTestService(t, &received)
	defer server.Close()

	config, err := service.GetConfig(context.Background())
	if err != nil || config.DataURL != server.URL || config.AssetsURL != "https://cdn/assets/3.3.9_abc" {
		t.Fatalf("unexpected config: %+v, %v", config, err)
	}

	version, err := service.GetBuildVersion(context.Background(), config.DataURL)
	if err != nil || version != "1.74.8" {
		t.Errorf("unexpected build version: %q, %v", version, err)
	}

	for _, req := range received {
		if req.userAgent != "KaellyBot-Gamedata/test" {
			t.Errorf("%s sent without the configured user agent: %q", req.path, req.userAgent)
		}
	}
}

func TestGetTableIsAPost(t *testing.T) {
	var received []request
	service, server := newTestService(t, &received)
	defer server.Close()

	body, err := service.GetTable(context.Background(), server.URL, "Jobs", "fr")
	if err != nil || string(body) != `{"26":{"id":26,"nameId":"Alchimiste"}}` {
		t.Fatalf("unexpected table: %s, %v", body, err)
	}

	req := received[len(received)-1]
	if req.method != http.MethodPost || req.query != "lang=fr" ||
		req.body["class"] != "Jobs" || req.body["lang"] != "fr" {
		t.Errorf("unexpected request: %+v", req)
	}
}

func TestUnexpectedStatus(t *testing.T) {
	var received []request
	service, server := newTestService(t, &received)
	defer server.Close()

	if _, err := service.GetBuildVersion(context.Background(), server.URL+"/missing"); !errors.Is(
		err, ErrUnexpectedStatus) {
		t.Errorf("expected ErrUnexpectedStatus, got %v", err)
	}
}
