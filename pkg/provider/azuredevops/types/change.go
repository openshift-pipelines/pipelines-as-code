package azuredevops

type Item struct {
	CommitID         string `json:"commitId"`
	GitObjectType    string `json:"gitObjectType"`
	IsFolder         bool   `json:"isFolder"`
	ObjectID         string `json:"objectId"`
	OriginalObjectID string `json:"originalObjectId"`
	Path             string `json:"path"`
	URL              string `json:"url"`
}

type Change struct {
	ChangeType string `json:"changeType"`
	Item       Item   `json:"item"`
}
