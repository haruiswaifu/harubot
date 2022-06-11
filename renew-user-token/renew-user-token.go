package renewusertoken

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type secrets struct {
	Username           string `json:"username"`
	OauthKey           string `json:"oauth-key"`
	TwitchAccessToken  string `json:"twitch-access-token"`
	TwitchClientID     string `json:"twitch-client-id"`
	TwitchClientSecret string `json:"twitch-client-secret"`
	TwitchRefreshToken string `json:"twitch-refresh-token"`
}

type tokenRefreshResponse struct {
	AccessToken string `json:"access_token"`
}

func RoutinelyRefreshToken(interval time.Duration) {
	for {
		refreshToken()
		time.Sleep(interval)
	}
}

func refreshToken() {
	secretsBytes, err := ioutil.ReadFile("./secrets.json")
	if err != nil {
		log.Fatalln("failed to read secrets")
	}
	s := &secrets{}
	err = json.Unmarshal(secretsBytes, s)
	if err != nil {
		log.Fatalln("failed to unmarshal secrets")
	}

	tokenURL := "https://id.twitch.tv/oauth2/token"
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", s.TwitchRefreshToken)
	data.Set("client_id", s.TwitchClientID)
	data.Set("client_secret", s.TwitchClientSecret)
	request, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		log.Fatalf("failed to make refresh token request: %s", err)
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.Fatalf("failed to do refresh token request: %s", err)
		// TODO: retry
	}
	if response.StatusCode != http.StatusOK {
		log.Fatalf("failed to do refresh token request: %s", response.Status)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("failed to read token request response: %s", err)
	}
	responseStruct := tokenRefreshResponse{}
	err = json.Unmarshal(body, &responseStruct)
	if err != nil {
		log.Fatalf("failed to unmarshal token request response: %s", err)
	}

	s.TwitchAccessToken = responseStruct.AccessToken

	marshalledSecrets, err := json.Marshal(s)
	if err != nil {
		log.Errorf("failed to marshal secrets: %s", err)
	}
	err = ioutil.WriteFile("./secrets.json", marshalledSecrets, 0644)
	if err != nil {
		log.Errorf("failed to write secrets: %s", err)
	}
}
