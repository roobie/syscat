package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/roobie/syscat/hello"
	"github.com/roobie/syscat/security"
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
var ctx = context.Background()
var conf = &oauth2.Config{
	ClientID:     os.Getenv("SYSCAT_GH_CLIENT_ID"),
	ClientSecret: os.Getenv("SYSCAT_GH_CLIENT_SECRET"),
	Scopes:       []string{"openid", "profile"},
	Endpoint:     github.Endpoint,
}

func main() {
	router := mux.NewRouter()

	fmt.Println(hello.BuildHello())

	router.HandleFunc("/", rootHandler)
	router.HandleFunc("/connect/github/callback", githubCallbackHandler)
	http.Handle("/", router)

	err := http.ListenAndServeTLS("syscat.lan:8443", cer, key, nil)
	log.Fatal(err)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	correlationId, err := security.MakeUUID()
	if err != nil {
		respondWithError(ErrorContext{
			w:             w,
			r:             r,
			statusCode:    http.StatusInternalServerError,
			correlationId: correlationId,
			message:       "Could not create a UUID",
			err:           err,
		})
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
}

func githubCallbackHandler(w http.ResponseWriter, r *http.Request) {
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

	randStr, err := security.GenerateRandomString(32)
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
}

// Writes to and closes `w`, so it cannot be altered elsewhere.
func loginAtIdP(conf *oauth2.Config, w http.ResponseWriter, r *http.Request) {
	nonce, err := security.GenerateRandomBytes(64)
	if err != nil {
		log.Println("Could not generate HMAC")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	state := security.ConstructMAC(nonce, []byte(appSecret))
	authCodeUrl := conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, authCodeUrl, http.StatusTemporaryRedirect)
}

type ErrorContext struct {
	w             http.ResponseWriter
	r             *http.Request
	statusCode    int
	correlationId string
	message       string
	err           error
}

func respondWithError(ectx ErrorContext) {
	msgOut := fmt.Sprintf("[%s] %d %s", ectx.correlationId, ectx.statusCode, ectx.message)
	log.Println(msgOut)
	// if isDevelopment {
	if ectx.err != nil {
		http.Error(ectx.w, fmt.Sprintf("%s\n%s", msgOut, ectx.err.Error()), ectx.statusCode)
	} else {
		http.Error(ectx.w, msgOut, ectx.statusCode)
	}
	// } else {...}
}
