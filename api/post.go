package api

//Post represents a 4chan post from a thread, including the OP, as it comes from the API
type Post struct {
	No            int64   `json:"no"`
	Resto         int64   `json:"resto"`
	Sticky        *int8   `json:"sticky"`
	Closed        *int8   `json:"closed"`
	Time          int32   `json:"time"`
	Name          *string `json:"name"`
	Trip          *string `json:"trip"`
	ID            *string `json:"id"`
	Capcode       *string `json:"capcode"`
	Country       *string `json:"country"`
	BoardFlag     *string `json:"board_flag"`
	Sub           *string `json:"sub"`
	Com           *string `json:"com"`
	Tim           *int64  `json:"tim"`
	Filename      *string `json:"filename"`
	Ext           *string `json:"ext"`
	Fsize         *int    `json:"fsize"`
	MD5           *string `json:"md5"`
	W             *int16  `json:"w"`
	H             *int16  `json:"h"`
	TnW           *int16  `json:"tn_w"`
	TnH           *int16  `json:"tn_h"`
	FileDeleted   *int8   `json:"filedeleted"`
	Spoiler       *int8   `json:"spoiler"`
	CustomSpoiler *int8   `json:"custom_spoiler"`
	Tag           *string `json:"tag"`
	Since4Pass    *int16  `json:"since4pass"`
	UniqueIPs     *int16  `json:"unique_ips"`
}
