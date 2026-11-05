package auth

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"golang.org/x/oauth2"
)

// profile is what a provider says about a person.
type profile struct {
	subject string // the provider's id for them: stable, never reused
	name    string // a name to show
	avatar  string // the address of their picture at the provider, or "" (already checked by safeAvatar)
}

// A provider says where to send people, where to trade the code for a token,
// and how to read who they are from the provider's own API.
type provider struct {
	name, label string
	endpoint    oauth2.Endpoint
	scopes      []string
	userURL     string
	parse       func(body []byte) (profile, error)
}

const avatarSize = "128" // pixels: the biggest an avatar is shown here is well under this

// safeAvatar keeps an address only if it is https and on one of the hosts a provider serves pictures from,
// so what is stored (and later loaded by every player's browser) cannot point anywhere else.
func safeAvatar(raw string, hosts ...string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return ""
	}
	for _, h := range hosts {
		if u.Host == h {
			return u.String()
		}
	}
	return ""
}

// Discord signs in with the "identify" scope only: id, name and picture, no email.
func discord() provider {
	const cdn = "cdn.discordapp.com"
	return provider{
		name: "discord", label: "Discord",
		endpoint: oauth2.Endpoint{AuthURL: "https://discord.com/oauth2/authorize", TokenURL: "https://discord.com/api/oauth2/token"},
		scopes:   []string{"identify"},
		userURL:  "https://discord.com/api/users/@me",
		parse: func(body []byte) (profile, error) {
			var u struct {
				ID            string  `json:"id"`
				Username      string  `json:"username"`
				GlobalName    string  `json:"global_name"`
				Avatar        *string `json:"avatar"` // a hash, not an address; null when they have none
				Discriminator string  `json:"discriminator"`
			}
			if err := json.Unmarshal(body, &u); err != nil || u.ID == "" {
				return profile{}, fmt.Errorf("discord: no user id in the reply")
			}
			p := profile{subject: u.ID, name: u.GlobalName}
			if p.name == "" {
				p.name = u.Username
			}
			if u.Avatar != nil && *u.Avatar != "" {
				// png even for an animated avatar (its hash starts "a_"): the first frame, which is all a chip needs.
				p.avatar = safeAvatar(fmt.Sprintf("https://%s/avatars/%s/%s.png?size=%s", cdn, u.ID, *u.Avatar, avatarSize), cdn)
			} else {
				p.avatar = safeAvatar(fmt.Sprintf("https://%s/embed/avatars/%d.png", cdn, defaultAvatar(u.ID, u.Discriminator)), cdn)
			}
			return p, nil
		},
	}
}

// defaultAvatar is which of Discord's stock pictures a person without one gets: by id for the current
// usernames (discriminator "0"), by discriminator for the old name#1234 ones.
func defaultAvatar(id, discriminator string) int {
	if d, err := strconv.Atoi(discriminator); err == nil && d > 0 {
		return d % 5
	}
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0
	}
	return int((n >> 22) % 6)
}

// GitHub signs in with no scope at all, which still allows reading the public profile.
func github() provider {
	return provider{
		name: "github", label: "GitHub",
		endpoint: oauth2.Endpoint{AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token"},
		userURL:  "https://api.github.com/user",
		parse: func(body []byte) (profile, error) {
			var u struct {
				ID        int64  `json:"id"`
				Login     string `json:"login"`
				Name      string `json:"name"`
				AvatarURL string `json:"avatar_url"`
			}
			if err := json.Unmarshal(body, &u); err != nil || u.ID == 0 {
				return profile{}, fmt.Errorf("github: no user id in the reply")
			}
			// The numeric id never changes; the login can.
			p := profile{subject: strconv.FormatInt(u.ID, 10), name: u.Name}
			if p.name == "" {
				p.name = u.Login
			}
			if a, err := url.Parse(u.AvatarURL); err == nil {
				q := a.Query()
				q.Set("s", avatarSize)
				a.RawQuery = q.Encode()
				p.avatar = safeAvatar(a.String(), "avatars.githubusercontent.com")
			}
			return p, nil
		},
	}
}
