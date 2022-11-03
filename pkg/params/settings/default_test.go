package settings

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestSetDefaults(t *testing.T) {
	config := make(map[string]string)
	SetDefaults(config)
	assert.Equal(t, config[RemoteTasksKey], remoteTasksDefaultValue)
	assert.Equal(t, config[SecretAutoCreateKey], secretAutoCreateDefaultValue)
	assert.Equal(t, config[BitbucketCloudCheckSourceIPKey], bitbucketCloudCheckSourceIPDefaultValue)
	assert.Equal(t, config[ApplicationNameKey], PACApplicationNameDefaultValue)
	assert.Equal(t, config[HubURLKey], HubURLDefaultValue)
	assert.Equal(t, config[HubCatalogNameKey], hubCatalogNameDefaultValue)
}
