package azuredevops

type Item struct {
	CommitId         string `json:"commitId"`
	GitObjectType    string `json:"gitObjectType"`
	IsFolder         bool   `json:"isFolder"`
	ObjectId         string `json:"objectId"`
	OriginalObjectId string `json:"originalObjectId"`
	Path             string `json:"path"`
	URL              string `json:"url"`
}

type Change struct {
	ChangeType string `json:"changeType"`
	Item       Item   `json:"item"`
}
