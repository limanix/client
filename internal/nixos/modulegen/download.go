package modulegen

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/limanix/client/internal/nixos/catalog"
)

func download(ctx context.Context, version string) (data []byte, failure error) {
	address := "https://" + catalog.Repository + "/archive/refs/tags/" + url.PathEscape(version) + ".tar.gz"
	response, err := get(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("download %s tag %s: %w", catalog.Repository, version, err)
	}
	defer func() {
		failure = errors.Join(failure, response.Body.Close())
	}()

	return pack(ctx, response.Body, version)
}

func get(ctx context.Context, address string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 2 * time.Minute,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if request.URL.Scheme != "https" || len(via) >= 10 {
				return fmt.Errorf("refused redirect to %s", request.URL)
			}
			return nil
		},
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, errors.Join(fmt.Errorf("HTTP %s", response.Status), response.Body.Close())
	}

	return response, nil
}
