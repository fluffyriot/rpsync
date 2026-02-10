// SPDX-License-Identifier: AGPL-3.0-only
package authhelp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
)

func GenerateFacebookConfig(appID, appSecret, callbackURL string) *oauth2.Config {
	facebookOAuthConfig := &oauth2.Config{
		ClientID:     appID,
		ClientSecret: appSecret,
		RedirectURL:  callbackURL,
		Scopes:       []string{"instagram_basic", "pages_show_list", "instagram_manage_comments", "pages_read_engagement", "instagram_manage_insights"},
		Endpoint:     facebook.Endpoint,
	}
	return facebookOAuthConfig
}

func OauthTokenToString(token *oauth2.Token) (string, error) {
	tokenM, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	return string(tokenM), nil
}

func ExchangeLongLivedToken(shortLivedToken string, config *oauth2.Config) (string, error) {
	endpoint := "https://graph.facebook.com/v24.0/oauth/access_token"
	params := url.Values{}
	params.Add("grant_type", "fb_exchange_token")
	params.Add("client_id", config.ClientID)
	params.Add("client_secret", config.ClientSecret)
	params.Add("fb_exchange_token", shortLivedToken)

	resp, err := http.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var res struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return "", err
	}

	return res.AccessToken, nil
}
