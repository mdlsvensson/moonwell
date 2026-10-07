package env

import (
	"context"
	"io"
	"net/http"
)

// FetchFunc downloads an address: the response's status and its whole body. A response with a failing status is
// not an error; an error means that no response came, or that ctx was cancelled. Tests replace it.
type FetchFunc func(ctx context.Context, url string) (status int, body []byte, err error)

// HTTPFetch returns the FetchFunc that downloads with client.
func HTTPFetch(client *http.Client) FetchFunc {
	return func(ctx context.Context, url string) (int, []byte, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return 0, nil, err
		}
		response, err := client.Do(request)
		if err != nil {
			return 0, nil, err
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return 0, nil, err
		}
		return response.StatusCode, body, nil
	}
}
