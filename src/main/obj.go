package main

type plurkerObj struct {
	UserInfo userInfoObj `json:"user_info"`
}

type userInfoObj struct {
	UserID   int     `json:"uid"`
	DispName string  `json:"display_name"`
	NickName string  `json:"nick_name"`
	FullName string  `json:"full_name"`
	Karma    float32 `json:"karma"`
	ID       int     `json:"id"`
}

type plurkObj struct {
	PlurkID    int    `json:"plurk_id"`
	IsUnread   int    `json:"is_unread"`
	Responded  int    `json:"responded"`
	UserID     int    `json:"user_id"`
	OwnerID    int    `json:"owner_id"`
	Posted     string `json:"posted"`
	Content    string `json:"content"`
	ContentRaw string `json:"content_raw"`
	ID         int    `json:"id"`
}

type plurksObj struct {
	Plurks        []plurkObj `json:"plurks"`
	ResponseCount int        `json:"response_count"`
	ResponseSeen  int        `json:"responses_seen"`
	Responses     []plurkObj `json:"responses"`
}
