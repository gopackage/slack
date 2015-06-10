package auth

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

// VerifyToken determines of the provided token is valid
func VerifyToken(token string) (bool, error) {
	resp, err := http.Get("https://slack.com/api/auth.test?token=" + token)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	var r Response
	err = json.Unmarshal(body, &r)
	if err != nil {
		return false, err
	}
	return r.Ok, nil
}

// Response encapsulates the `auth.test` Slack web API response.
// {
//   "ok":true,
//   "url":"https:\/\/intellimatics.slack.com\/",
//   "team":"Intellimatics",
//   "user":"bitbot",
//   "team_id":"T024FL887",
//   "user_id":"U03AHNBPC"
// }
type Response struct {
	Ok     bool   `json:"ok"`
	URL    string `json:"url"`
	Team   string `json:"team"`
	User   string `json:"user"`
	TeamID string `json:"team_id"`
	UserID string `json:"user_id"`
}
