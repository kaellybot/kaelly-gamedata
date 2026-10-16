package touch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"

	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/kaellybot/kaelly-gamedata/models/dtos"
	"github.com/spf13/viper"
)

func New() *Impl {
	return &Impl{
		configURL: viper.GetString(constants.TouchConfigURL),
		userAgent: viper.GetString(constants.TouchUserAgent),
		client:    &http.Client{Timeout: viper.GetDuration(constants.HTTPTimeout)},
	}
}

func (service *Impl) GetConfig(ctx context.Context) (*dtos.TouchConfig, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, service.configURL, nil)
	if err != nil {
		return nil, err
	}

	body, err := service.do(req)
	if err != nil {
		return nil, err
	}

	var config dtos.TouchConfig
	if errJSON := json.Unmarshal(body, &config); errJSON != nil {
		return nil, errJSON
	}

	if config.DataURL == "" {
		return nil, ErrNoDataURL
	}

	return &config, nil
}

// GetBuildVersion reads window.buildVersion from the client bundle.
func (service *Impl) GetBuildVersion(ctx context.Context, dataURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dataURL+buildScriptPath, nil)
	if err != nil {
		return "", err
	}

	body, err := service.do(req)
	if err != nil {
		return "", err
	}

	match := regexp.MustCompile(`window\.buildVersion\s*=\s*"([^"]+)"`).FindSubmatch(body)
	if match == nil {
		return "", ErrNoBuildVersion
	}

	return string(match[1]), nil
}

// GetTable downloads one client table, already translated in the language.
// The route only answers POST.
func (service *Impl) GetTable(ctx context.Context, dataURL, class, language string) ([]byte, error) {
	payload, err := json.Marshal(map[string]string{"class": class, "lang": language})
	if err != nil {
		return nil, err
	}

	tableURL := fmt.Sprintf("%s%s?lang=%s", dataURL, dataMapPath, url.QueryEscape(language))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tableURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return service.do(req)
}

func (service *Impl) do(req *http.Request) ([]byte, error) {
	req.Header.Set("User-Agent", service.userAgent)
	resp, err := service.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s on %s", ErrUnexpectedStatus, resp.Status, req.URL.Redacted())
	}

	return io.ReadAll(resp.Body)
}
