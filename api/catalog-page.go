package api

//CatalogPage is a 4chan page as it comes from the /catalog endpoint
type CatalogPage struct {
	Page    uint8           `json:"page"`
	Threads []CatalogThread `json:"threads"`
}
