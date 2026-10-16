package touch

import (
	"context"
	"errors"
	"net/http"

	"github.com/kaellybot/kaelly-gamedata/models/dtos"
)

const (
	buildScriptPath = "/build/script.js"
	dataMapPath     = "/data/map"
)

var (
	ErrUnexpectedStatus = errors.New("unexpected HTTP status")
	ErrNoBuildVersion   = errors.New("no buildVersion found in the client script")
	ErrNoDataURL        = errors.New("no dataUrl in the client config")
)

type Service interface {
	GetConfig(ctx context.Context) (*dtos.TouchConfig, error)
	GetBuildVersion(ctx context.Context, dataURL string) (string, error)
	GetTable(ctx context.Context, dataURL, class, language string) ([]byte, error)
}

type Impl struct {
	configURL string
	userAgent string
	client    *http.Client
}
