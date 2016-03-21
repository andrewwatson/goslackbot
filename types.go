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
	ID            string `json:"id"`
	Name          string `json:"name"`
	IsArchived    bool   `json:"is_archived"`
	LastMessageID uint64 `json:"-"`
}

type SlackRTMResponse struct {
	Ok       bool                 `json:"ok"`
	Error    string               `json:"error"`
	Url      string               `json:"url"`
	Self     SlackRTMResponseSelf `json:"self"`
	Users    []SlackUser          `json:"users"`
	Channels []SlackChannel       `json:"channels"`
	IMs      []SlackChannel       `json:"channels"`
	MPIMs    []SlackChannel       `jsonL:"mpims"`
	Groups   []SlackChannel       `json:"groups"`
	Teams    []SlackTeam          `json:"teams"`
}

type SlackRTMResponseSelf struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type SlackMessage struct {
	Id        uint64           `json:"id"`
	Type      string           `json:"type"`
	SubType   string           `json:"sub_type"`
	Channel   string           `json:"channel"`
	Text      string           `json:"text"`
	User      string           `json:"user"`
	ReplyTo   uint64           `json:"reply_to, omitempty"`
	TimeStamp string           `json:"ts, omitempty"`
	Item      SlackMessageItem `json:"item, omitempty"`
	Name      string           `json:"name, omitempty"`
	Reaction  string           `json:"reaction, omitempty"`
}

type SlackMessageItem struct {
	Type      string `json:"type"`
	Channel   string `json:"channel"`
	TimeStamp string `json:"ts"`
}

type SlackTeam struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	EmailDomain string `json:"email_domain"`
	Domain      string `json:"domain"`
}
