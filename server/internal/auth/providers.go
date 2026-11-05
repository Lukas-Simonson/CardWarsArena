package auth

import (
	"encoding/json"
	"fmt"
	"strconv"

	"golang.org/x/oauth2"
)

// A provider says where to send people, where to trade the code for a token,
// and how to read who they are from the provider's own API.
type provider struct {
	name, label string
	endpoint    oauth2.Endpoint
	scopes      []string
	userURL     string
	// parse reads the provider's id for the person (stable, never reused) and a name to show.
	parse func(body []byte) (subject, name string, err error)
}

// Discord signs in with the "identify" scope only: id and name, no email.
func discord() provider {
	return provider{
		name: "discord", label: "Discord",
		endpoint: oauth2.Endpoint{AuthURL: "https://discord.com/oauth2/authorize", TokenURL: "https://discord.com/api/oauth2/token"},
		scopes:   []string{"identify"},
		userURL:  "https://discord.com/api/users/@me",
		parse: func(body []byte) (string, string, error) {
			var u struct {
				ID         string `json:"id"`
				Username   string `json:"username"`
				GlobalName string `json:"global_name"`
			}
			if err := json.Unmarshal(body, &u); err != nil || u.ID == "" {
				return "", "", fmt.Errorf("discord: no user id in the reply")
			}
			if u.GlobalName != "" {
				return u.ID, u.GlobalName, nil
			}
			return u.ID, u.Username, nil
		},
	}
}

// GitHub signs in with no scope at all, which still allows reading the public profile.
func github() provider {
	return provider{
		name: "github", label: "GitHub",
		endpoint: oauth2.Endpoint{AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token"},
		userURL:  "https://api.github.com/user",
		parse: func(body []byte) (string, string, error) {
			var u struct {
				ID    int64  `json:"id"`
				Login string `json:"login"`
				Name  string `json:"name"`
			}
			if err := json.Unmarshal(body, &u); err != nil || u.ID == 0 {
				return "", "", fmt.Errorf("github: no user id in the reply")
			}
			// The numeric id never changes; the login can.
			if u.Name != "" {
				return strconv.FormatInt(u.ID, 10), u.Name, nil
			}
			return strconv.FormatInt(u.ID, 10), u.Login, nil
		},
	}
}
