package shared

import (
	"strings"

	"github.com/cli/cli/v2/internal/config"
)

func AuthTokenWriteable(authCfg *config.AuthConfig, hostname string) (string, bool) {
	token, src := authCfg.ActiveToken(hostname)
	return src, (token == "" || !strings.HasSuffix(src, "_TOKEN"))
}
