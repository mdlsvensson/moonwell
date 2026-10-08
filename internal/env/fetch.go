package env

import (
	"context"
	"io"
	"net/http"
)

type FetchFunc func(ctx context.Context, url string) (status int, body []byte, err error)

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
