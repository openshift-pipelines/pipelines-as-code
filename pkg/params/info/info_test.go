package info

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/stretchr/testify/assert"
)

func TestNewInfo(t *testing.T) {
	info := NewInfo()
	assert.Equal(t, info.Pac.ApplicationName, "Pipelines as Code CI")

	value, ok := info.Pac.HubCatalogs.Load("default")
	assert.Equal(t, true, ok)

	catalog, ok := value.(settings.HubCatalog)
	assert.Equal(t, true, ok)
	assert.Equal(t, catalog.Index, "default")
	assert.Equal(t, catalog.Name, settings.HubCatalogNameDefaultValue)
}
