package api

//Thread represents a 4chan thread as it comes directly from the API's thread endpoint
type Thread struct {
	Posts []Post `json:"posts"`
}
