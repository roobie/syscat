package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/gorilla/sessions"
	"github.com/roobie/syscat/hello"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

var cer string = os.Getenv("SYSCAT_CER")
var key string = os.Getenv("SYSCAT_KEY")
var appSecret string = os.Getenv("SYSCAT_APP_SECRET")
var hostname string = os.Getenv("SYSCAT_HOSTNAME")

// Actual constants
const appSessionName string = "syscat-session"
const authKey string = "syscat-auth"
const correlationIdKey string = "syscat-correlation-id"

var store = sessions.NewCookieStore([]byte(appSecret))
var tokenMap map[string]*oauth2.Token = make(map[string]*oauth2.Token)

// Writes to and closes `w`, so it cannot be altered elsewhere.
func loginAtIdP(conf *oauth2.Config, w http.ResponseWriter, r *http.Request) {
	nonce, err := GenerateRandomBytes(64)
	if err != nil {
		log.Println("Could not generate HMAC")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	state := ConstructMAC(nonce, []byte(appSecret))
	authCodeUrl := conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, authCodeUrl, http.StatusTemporaryRedirect)
}

func RespondWithError(w http.ResponseWriter, r *http.Request, statusCode int, correlationId string, message string, err error) {
	msgOut := fmt.Sprintf("[%s] %d %s", correlationId, statusCode, message)
	log.Println(msgOut)
	// if isDevelopment {
	if err != nil {
		http.Error(w, fmt.Sprintf("%s\n%s", msgOut, err.Error()), http.StatusInternalServerError)
	} else {
		http.Error(w, msgOut, http.StatusInternalServerError)
	}
	// } else {...}
}

func main() {
	ctx := context.Background()

	conf := &oauth2.Config{
		ClientID:     os.Getenv("SYSCAT_GH_CLIENT_ID"),
		ClientSecret: os.Getenv("SYSCAT_GH_CLIENT_SECRET"),
		Scopes:       []string{"openid", "profile"},
		Endpoint:     github.Endpoint,
	}

	fmt.Println(hello.BuildHello())

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		correlationId, err := MakeUUID()
		if err != nil {
			RespondWithError(w, r, http.StatusInternalServerError, correlationId, "Could not create a UUID", err)
			return
		}
		session, err := store.Get(r, appSessionName)
		if err != nil {
			log.Println("Could not retrieve session")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if session.Values[authKey] != nil {
			tok := tokenMap[session.Values[authKey].(string)]
			client := conf.Client(ctx, tok)
			response, err := client.Get("https://api.github.com/user")
			if err != nil {
				log.Printf("Failed to query https://api.github.com/user due to [%s]\n", err.Error())
				if strings.Contains(err.Error(), "token expired") {
					loginAtIdP(conf, w, r)
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				log.Println("Failed read response from https://api.github.com/user")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			fmt.Fprintf(w, "Body: %s\n", string(body))
		} else {
			loginAtIdP(conf, w, r)
		}
	})

	http.HandleFunc("/connect/github/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		code, ok := query["code"]
		if !ok || len(code[0]) < 1 {
			http.Error(w, "Invalid response from identity provider", http.StatusInternalServerError)
			return
		}
		tok, err := conf.Exchange(ctx, code[0])
		if err != nil {
			log.Fatal(err)
		}

		session, err := store.Get(r, appSessionName)
		if err != nil {
			log.Println("Could not retrieve session")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		randStr, err := GenerateRandomString(32)
		if err != nil {
			log.Println("Could not make random string")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tokenMap[randStr] = tok
		session.Values[authKey] = randStr
		err = session.Save(r, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	})

	err := http.ListenAndServeTLS("syscat.lan:8443", cer, key, nil)
	log.Fatal(err)
}

// GenerateRandomBytes returns securely generated random bytes.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

// GenerateRandomString returns a URL-safe, base64 encoded
// securely generated random string.
func GenerateRandomString(s int) (string, error) {
	b, err := GenerateRandomBytes(s)
	return base64.URLEncoding.EncodeToString(b), err
}

func ConstructMAC(message, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

// ValidMAC reports whether messageMAC is a valid HMAC tag for message.
func ValidMAC(message, messageMAC, key []byte) bool {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(messageMAC, expectedMAC)
}

func MakeUUID() (string, error) {
	b, err := GenerateRandomBytes(16)
	if err != nil {
		return "", err
	}
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return uuid, nil
}
