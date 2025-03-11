package test

type Pagination struct {
	Start    int  `json:"start"`
	Size     int  `json:"size"`
	Limit    int  `json:"limit"`
	LastPage bool `json:"isLastPage"`
	NextPage int  `json:"nextPageStart"`
}

type DiffPath struct {
	ToString string `json:"toString"`
}

type DiffStat struct {
	Path    DiffPath  `json:"path"`
	SrcPath *DiffPath `json:"srcPath,omitempty"` // used in lib that's why leaving it here nil
	Type    string    `json:"type"`
}

type DiffStats struct {
	Pagination
	Values []*DiffStat
}
