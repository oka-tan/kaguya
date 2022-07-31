package api

//CatalogThread is a 4chan thread as it comes from the /catalog endpoint
type CatalogThread struct {
	No           int64  `json:"no"`
	LastModified uint64 `json:"last_modified"`
}
