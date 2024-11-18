package bitbucketserver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

func callAPI(ctx context.Context, endpointURL, method string, fields map[string]string) ([]byte, error) {
	req, err := createRequest(ctx, endpointURL, method, fields)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode > 300 {
		return nil, fmt.Errorf("error status code: %d", resp.StatusCode)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	return responseBody, nil
}

func createRequest(ctx context.Context, endpointURL, method string, fields map[string]string) (*http.Request, error) {
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	if len(fields) > 0 {
		for field, value := range fields {
			err := writer.WriteField(field, value)
			if err != nil {
				return nil, fmt.Errorf("error writing field %s to multipart data: %w", field, err)
			}
		}

		err := writer.Close()
		if err != nil {
			return nil, fmt.Errorf("error closing writer: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, endpointURL, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	if len(fields) > 0 {
		req.Header.Set("Content-Type", writer.FormDataContentType())
	}

	bitbucketServerUser := os.Getenv("TEST_BITBUCKET_SERVER_USER")
	bitbucketServerToken := os.Getenv("TEST_BITBUCKET_SERVER_TOKEN")
	req.SetBasicAuth(bitbucketServerUser, bitbucketServerToken)

	return req, nil
}
