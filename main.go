package main

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/joho/godotenv"
)

func main() {

	go func() {
		http.HandleFunc("/login", spotify_login)
		auth_code := make(chan string)
		go pass_callback(auth_code)
		handleCallback := spotify_callback(auth_code)
		http.HandleFunc("/callback", handleCallback)

		err := http.ListenAndServe("0.0.0.0:3000", nil)

		if err != nil {
			log.Fatal(err)
		}
	}()

	godotenv.Load(".env")

	if os.Getenv("TWITTER_CK") == "" || os.Getenv("TWITTER_CS") == "" || os.Getenv("TWITTER_AT") == "" || os.Getenv("TWITTER_AS") == "" || os.Getenv("SPOTIFY_CLIENT_ID") == "" || os.Getenv("SPOTIFY_CLIENT_SECRET") == "" {
		log.Fatal("Failed to load credentials.")
	} else if os.Getenv("SPOTIFY_REFRESH_TOKEN") == "" {
		fmt.Println("`SPOTIFY_REFRESH_TOKEN` is not set. Please click the URL below.")
		values := url.Values{}
		values.Add("client_id", os.Getenv("SPOTIFY_CLIENT_ID"))
		values.Add("response_type", "code")
		values.Add("redirect_uri", "http://localhost:3000/callback")
		fmt.Println("https://accounts.spotify.com/authorize?" + values.Encode())
	}

	consumerKey := os.Getenv("TWITTER_CK")
	consumerSecret := os.Getenv("TWITTER_CS")
	accessToken := os.Getenv("TWITTER_AT")
	accessSecret := os.Getenv("TWITTER_AS")

	if consumerKey == "" || consumerSecret == "" || accessToken == "" || accessSecret == "" {
		log.Fatal("Consumer key/secret and Access token/secret required")
	}

	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)
	// OAuth1 http.Client will automatically authorize Requests
	httpClient := config.Client(oauth1.NoContext, token)

	// Twitter client
	client := twitter.NewClient(httpClient)
	verifyParams := &twitter.AccountVerifyParams{
		SkipStatus:   twitter.Bool(true),
		IncludeEmail: twitter.Bool(true),
	}
	user, _, _ := client.Accounts.VerifyCredentials(verifyParams)
	fmt.Printf("✅ Logged in as %s (@%s) on Twitter.\n", user.Name, user.ScreenName)

	last_title := ""

	ticker := time.NewTicker(60 * time.Second)

	for {
		select {
		case <-ticker.C:
			is_playing, title, artist, album, url, progress := get_spotify_np()
			if is_playing {
				if last_title == "" || title != last_title {
					if progress > 5000 {
						message := fmt.Sprintf("🎵 #NowPlaying #np: %s / %s (%s)\n%s", title, artist, album, url)
						fmt.Println(message)
						profileParams := &twitter.AccountUpdateProfileParams{
							Description: message,
						}
						client.Accounts.UpdateProfile(profileParams)

						last_title = title
					}
				}
			} else {
				title, artist, album = "", "", ""
				if last_title != "" {
					profileParams := &twitter.AccountUpdateProfileParams{
						Description: os.Getenv("BIO_DEFAULT"),
					}
					client.Accounts.UpdateProfile(profileParams)
				}
			}

		}
	}
}

func spotify_login(w http.ResponseWriter, req *http.Request) {
	values := url.Values{}
	values.Add("client_id", os.Getenv("SPOTIFY_CLIENT_ID"))
	values.Add("response_type", "code")
	values.Add("redirect_uri", "http://localhost:3000/callback")

	http.Redirect(w, req, "https://accounts.spotify.com/authorize?"+values.Encode(), http.StatusFound)
}

func spotify_callback(auth_code chan string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		query := req.URL.Query().Get("code")
		auth_code <- query

		w.WriteHeader(200)
		w.Header().Set("Content-Type", "text/html; charset=utf8")

		w.Write([]byte("処理が完了しました。この画面を閉じることができます。\nnp2bio を再起動してください。"))

	}
}

func pass_callback(auth_code chan string) {
	for item := range auth_code {
		save_refresh_token(item)
	}
}

func save_refresh_token(auth_code string) {
	values := make(url.Values)
	values.Set("grant_type", "authorization_code")
	values.Set("code", auth_code)
	values.Set("redirect_uri", "http://localhost:3000/callback")
	req, err := http.NewRequest(http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader(values.Encode()))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", b64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", os.Getenv("SPOTIFY_CLIENT_ID"), os.Getenv("SPOTIFY_CLIENT_SECRET"))))))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var jsonObj interface{}
	if err := json.Unmarshal(body, &jsonObj); err != nil {
		fmt.Println("Failed to parse JSON @ save refresh token: ", string(body))
		log.Fatal(err)
	}

	refresh_token := jsonObj.(map[string]interface{})["refresh_token"].(string)
	refresh_token_env, err := godotenv.Unmarshal(fmt.Sprintf("TWITTER_CK=%s\nTWITTER_CS=%s\nTWITTER_AT=%s\nTWITTER_AS=%s\nSPOTIFY_CLIENT_ID=%s\nSPOTIFY_CLIENT_SECRET=%s\nSPOTIFY_REFRESH_TOKEN=%s\n", os.Getenv("TWITTER_CK"), os.Getenv("TWITTER_CS"), os.Getenv("TWITTER_AT"), os.Getenv("TWITTER_AS"), os.Getenv("SPOTIFY_CLIENT_ID"), os.Getenv("SPOTIFY_CLIENT_SECRET"), refresh_token))

	if err != nil {
		log.Fatal(err)
	}
	err = godotenv.Write(refresh_token_env, "./.env")
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}

func get_spotify_access_token() string {
	values := make(url.Values)
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", os.Getenv("SPOTIFY_REFRESH_TOKEN"))

	req, err := http.NewRequest(http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader(values.Encode()))
	if err != nil {
		log.Fatal(err)
	}

	spotify_auth_string := b64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", os.Getenv("SPOTIFY_CLIENT_ID"), os.Getenv("SPOTIFY_CLIENT_SECRET"))))

	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", spotify_auth_string))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var jsonObj interface{}
	if err := json.Unmarshal(body, &jsonObj); err != nil {
		fmt.Println("Failed to parse json @ retrieve access token: \n", string(body))
		log.Fatal(err)
	}

	if isNil(jsonObj.(map[string]interface{})["access_token"]) {
		fmt.Println("Access token is null or not a valid object: \n", string(body))
		os.Exit(1)
	}

	return jsonObj.(map[string]interface{})["access_token"].(string)
}

func isNil(i interface{}) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(i).IsNil()
	}
	return false
}

func get_spotify_np() (is_playing bool, title string, artist string, album string, url string, progress float64) {
	req, err := http.NewRequest(http.MethodGet, "https://api.spotify.com/v1/me/player/currently-playing?market=JP", nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", get_spotify_access_token()))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var jsonObj interface{}
	if err := json.Unmarshal(body, &jsonObj); err != nil {
		fmt.Println("Failed to parse json @ retrieve Now Playing status: \n", string(body))
		if err.Error() == "unexpected end of JSON input" {
			fmt.Println("Warning: unexpected end of JSON input.")
			is_playing = false
			title, artist, album = "", "", ""
			return is_playing, title, artist, album, url, progress
		} else {
			log.Fatal(err)
		}
	}

	if isNil(jsonObj.(map[string]interface{})["is_playing"]) {
		fmt.Println("JSON object is null or not a valid object @ retrieve Now Playing Status: \n", string(body))
		os.Exit(1)
	}
	is_playing = jsonObj.(map[string]interface{})["is_playing"].(bool)

	if is_playing {
		title = jsonObj.(map[string]interface{})["item"].(map[string]interface{})["name"].(string)

		artists := jsonObj.(map[string]interface{})["item"].(map[string]interface{})["artists"]
		for i := 0; i < len(artists.([]interface{})); i++ {
			artist += artists.([]interface{})[i].(map[string]interface{})["name"].(string)
			if i < len(artists.([]interface{}))-1 {
				artist += ", "
			}
		}

		album = jsonObj.(map[string]interface{})["item"].(map[string]interface{})["album"].(map[string]interface{})["name"].(string)

		url = jsonObj.(map[string]interface{})["item"].(map[string]interface{})["external_urls"].(map[string]interface{})["spotify"].(string)

		progress = jsonObj.(map[string]interface{})["progress_ms"].(float64)
	} else {
		is_playing = false

		title, artist, album = "", "", ""
	}

	return is_playing, title, artist, album, url, progress
}
