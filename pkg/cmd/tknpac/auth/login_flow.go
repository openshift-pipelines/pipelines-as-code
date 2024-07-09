package auth

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cli/cli/v2/api"
	"github.com/cli/oauth"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/browser"
)

// DefaultGithubHostname is the domain name of the default GitHub instance.
const defaultGithubHostname = "github.com"

// DefaultGitlabHostname is the domain name of the default GitHub instance.
// const defaultGitlabHostname = "gitlab.com"

// DefaultBitbucketHostname is the domain name of the default GitHub instance.
// const defaultBitbucketHostname = "bitbucket.org"

// Localhost is the domain name of a local GitHub instance.
const localhost = "github.localhost"

// TenancyHost is the domain name of a tenancy GitHub instance.
const tenancyHost = "ghe.com"

var (
	// oauth app client ID.
	oauthClientID = "Ov23linjEIlP76Xz0qgC"
	// #nosec G101: Potential hardcoded credentials
	oauthClientSecret = "f38d05e5dcfa672ed0dd36ad81b98e263c68bab3"
)

func RunAuthFlow(oauthHost string, ioStreams *cli.IOStreams, notice string, additionalScopes []string, isInteractive bool) (string, string, error) {
	w := ioStreams.ErrOut
	cs := ioStreams.ColorScheme()

	httpClient := &http.Client{}
	// debugEnabled, debugValue := utils.IsDebugEnabled()
	// if debugEnabled {
	// 	logTraffic := strings.Contains(debugValue, "api")
	// 	httpClient.Transport = verboseLog(IO.ErrOut, logTraffic, IO.ColorEnabled())(httpClient.Transport)
	// }

	scopes := []string{"repo", "read:org", "gist"}
	scopes = append(scopes, additionalScopes...)

	callbackURI := "http://127.0.0.1/callback"
	if IsEnterprise(oauthHost) {
		// the OAuth app on Enterprise hosts is still registered with a legacy callback URL
		// see https://github.com/cli/cli/pull/222, https://github.com/cli/cli/pull/650
		callbackURI = "http://localhost/"
	}

	flow := &oauth.Flow{
		Host:         oauth.GitHubHost(HostPrefix(oauthHost)),
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		CallbackURI:  callbackURI,
		Scopes:       scopes,
		DisplayCode: func(code, _ string) error {
			fmt.Fprintf(w, "%s First copy your one-time code: %s\n", cs.Yellow("!"), cs.Bold(code))
			return nil
		},
		BrowseURL: func(authURL string) error {
			if u, err := url.Parse(authURL); err == nil {
				if u.Scheme != "http" && u.Scheme != "https" {
					return fmt.Errorf("invalid URL: %s", authURL)
				}
			} else {
				return err
			}

			if !isInteractive {
				fmt.Fprintf(w, "%s to continue in your web browser: %s\n", cs.Bold("Open this URL"), authURL)
				return nil
			}

			fmt.Fprintf(w, "%s to open %s in your browser... ", cs.Bold("Press Enter"), oauthHost)
			_ = waitForEnter(ioStreams.In)

			if err := browser.OpenWebBrowser(authURL); err != nil {
				fmt.Fprintf(w, "%s Failed opening a web browser at %s\n", cs.Red("!"), authURL)
				fmt.Fprintf(w, "  %s\n", err)
				fmt.Fprint(w, "  Please try entering the URL in your browser manually\n")
			}
			return nil
		},
		WriteSuccessHTML: func(w io.Writer) {
			fmt.Fprint(w, oauthSuccessPage)
		},
		HTTPClient: httpClient,
		Stdin:      ioStreams.In,
		Stdout:     w,
	}

	fmt.Fprintln(w, notice)

	token, err := flow.DetectFlow()
	if err != nil {
		return "", "", err
	}

	userLogin, err := getViewer(oauthHost, token.Token, ioStreams.ErrOut)
	if err != nil {
		return "", "", err
	}

	return token.Token, userLogin, nil
}

func waitForEnter(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Scan()
	return scanner.Err()
}

type cfg struct {
	token string
}

func (c cfg) ActiveToken(_ string) (string, string) {
	return c.token, "oauth_token"
}

func getViewer(hostname, token string, logWriter io.Writer) (string, error) {
	opts := api.HTTPClientOptions{
		Config: cfg{token: token},
		Log:    logWriter,
	}
	client, err := api.NewHTTPClient(opts)
	if err != nil {
		return "", err
	}

	return api.CurrentLoginName(api.NewClientFromHTTP(client), hostname)
}

// IsEnterprise reports whether a non-normalized host name looks like a GHE instance.
func IsEnterprise(h string) bool {
	normalizedHostName := NormalizeHostname(h)
	return normalizedHostName != defaultGithubHostname && normalizedHostName != localhost
}

// NormalizeHostname returns the canonical host name of a GitHub instance.
func NormalizeHostname(h string) string {
	hostname := strings.ToLower(h)
	if strings.HasSuffix(hostname, "."+defaultGithubHostname) {
		return defaultGithubHostname
	}
	if strings.HasSuffix(hostname, "."+localhost) {
		return localhost
	}
	if before, found := cutSuffix(hostname, "."+tenancyHost); found {
		idx := strings.LastIndex(before, ".")
		return fmt.Sprintf("%s.%s", before[idx+1:], tenancyHost)
	}
	return hostname
}

func HostPrefix(hostname string) string {
	if strings.EqualFold(hostname, localhost) {
		return fmt.Sprintf("http://%s/", hostname)
	}
	return fmt.Sprintf("https://%s/", hostname)
}

// Backport strings.CutSuffix from Go 1.20.
func cutSuffix(s, suffix string) (string, bool) {
	if !strings.HasSuffix(s, suffix) {
		return s, false
	}
	return s[:len(s)-len(suffix)], true
}

func scopesSentence(scopes []string) string {
	quoted := make([]string, len(scopes))
	for i, s := range scopes {
		quoted[i] = fmt.Sprintf("'%s'", s)
	}
	return strings.Join(quoted, ", ")
}
