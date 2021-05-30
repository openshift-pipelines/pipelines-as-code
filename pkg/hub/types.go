package hub

type resourceVersionDataResponseBody struct {
	// ID is the unique id of resource's version
	ID *uint `json:"id,omitempty"`
	// Version of resource
	Version *string `json:"version,omitempty"`
	// Display name of version
	DisplayName *string `json:"displayName,omitempty"`
	// Description of version
	Description *string `json:"description,omitempty"`
	// Minimum pipelines version the resource's version is compatible with
	MinPipelinesVersion *string `json:"minPipelinesVersion,omitempty"`
	// Raw URL of resource's yaml file of the version
	RawURL *string `json:"rawURL,omitempty"`
	// Web URL of resource's yaml file of the version
	WebURL *string `json:"webURL,omitempty"`
	// Timestamp when version was last updated
	UpdatedAt *string `json:"updatedAt,omitempty"`
}

type hubResourceResponseBody struct {
	// ID is the unique id of the resource
	ID *uint `json:"id,omitempty"`
	// Name of resource
	Name *string `json:"name,omitempty"`
	// Kind of resource
	Kind *string `json:"kind,omitempty"`
	// Latest version of resource
	LatestVersion *resourceVersionDataResponseBody `json:"latestVersion,omitempty"`
	// List of all versions of a resource
	Versions []*resourceVersionDataResponseBody `json:"versions,omitempty"`
}

type hubResource struct {
	Data *hubResourceResponseBody `json:"data,omitempty"`
}

type hubResourceVersion struct {
	Data *resourceVersionDataResponseBody `json:"data,omitempty"`
}
