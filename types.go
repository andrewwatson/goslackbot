package goslackbot

type SlackUser struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Profile      SlackUserProfile `json:"profile"`
	IsAdmin      bool             `json:"is_admin"`
	IsOwner      bool             `json:"is_owner"`
	IsRestricted bool             `json:"is_restricted"`
	Has2FA       bool             `json:"has_2fa"`
}

type SlackUserProfile struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	RealName  string `json:"real_name"`
	Email     string `json:"email"`
	Skype     string `json:"skype"`
	Phone     string `json:"phone"`
}

type SlackChannel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsArchived bool   `json:"is_archived"`
}

type SlackRTMResponse struct {
	Ok       bool                 `json:"ok"`
	Error    string               `json:"error"`
	Url      string               `json:"url"`
	Self     SlackRTMResponseSelf `json:"self"`
	Users    []SlackUser          `json:"users"`
	Channels []SlackChannel       `json:"channels"`
	MPIMs    []SlackChannel       `jsonL:"mpims"`
	Groups   []SlackChannel       `json:"groups"`
}

type SlackRTMResponseSelf struct {
	Id string `json:"id"`
}

type SlackMessage struct {
	Id      uint64 `json:"id"`
	Type    string `json:"type"`
	SubType string `json:"sub_type"`
	Channel string `json:"channel"`
	Text    string `json:"text"`
	User    string `json:"user"`
	ReplyTo uint64 `json:"reply_to, omitempty"`
}
