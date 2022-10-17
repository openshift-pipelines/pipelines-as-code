package payload

import (
	"bytes"
	"context"
	"crypto/hmac"
	"io"
	"os"
	"strings"

	//nolint:gosec
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
)

func Send(ctx context.Context, cs *params.Run, elURL, elWebHookSecret, githubURL, installationID string, event interface{}, eventType string) error {
	jeez, err := json.Marshal(event)
	if err != nil {
		return err
	}

	mac := hmac.New(sha1.New, []byte(elWebHookSecret))
	mac.Write(jeez)
	sha1secret := hex.EncodeToString(mac.Sum(nil))
	mac = hmac.New(sha256.New, []byte(elWebHookSecret))
	mac.Write(jeez)
	sha256secret := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, "POST", elURL, bytes.NewBuffer(jeez))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", eventType)
	req.Header.Set("X-Hub-Signature", fmt.Sprintf("sha1=%s", sha1secret))
	req.Header.Set("X-Hub-Signature-256", fmt.Sprintf("sha256=%s", sha256secret))
	req.Header.Set("X-GitHub-Hook-Installation-Target-Type", "integration")
	req.Header.Set("X-GitHub-Hook-Installation-Target-ID", installationID)
	hostURL := githubURL
	if strings.HasPrefix(hostURL, "http") {
		parsed, err := url.Parse(githubURL)
		if err != nil {
			return err
		}
		hostURL = parsed.Host
	}
	if hostURL != "github.com" {
		req.Header.Set("X-GitHub-Enterprise-Host", hostURL)
	}

	cs.Clients.Log.Infof("Sending a payload directly to the EL on %s: %s headers: %+v", os.Getenv("TEST_EL_URL"), string(jeez), req.Header)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	statusOK := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !statusOK {
		return fmt.Errorf("responses Error: %+d", resp.StatusCode)
	}
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	return err
}
