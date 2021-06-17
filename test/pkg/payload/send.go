package payload

import (
	"bytes"
	"context"
	"crypto/hmac"
	"os"

	// nolint:gosec
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
)

func Send(ctx context.Context, cs *cli.Clients, elURL, elWebHookSecret, githubURL, installationID string, event interface{}, eventType string) error {
	jeez, err := json.Marshal(event)
	if err != nil {
		return err
	}
	cs.Log.Infof("Sending a payload directly to the EL on %s: %s", os.Getenv("TEST_EL_URL"), string(jeez))

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
	u, err := url.Parse(githubURL)
	if err != nil {
		return err
	}
	req.Header.Set("X-GitHub-Enterprise-Host", u.Host)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	return err
}
