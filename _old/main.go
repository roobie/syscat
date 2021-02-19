package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
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
var macMap map[string][]byte = make(map[string][]byte)
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

	router.HandleFunc("/", rootHandler)
	router.HandleFunc("/error", errorHandler)
	router.HandleFunc("/connect/github/callback", githubCallbackHandler)

	router.Use(CorrelationIdMiddleWare)
	router.Use(LoggingMiddleware)

	err := http.ListenAndServeTLS("syscat.lan:8443", cer, key, handlers.CORS()(router))
	log.Fatal(err)
}

func CorrelationIdMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationId := ""
		if r.URL.Path == "/connect/github/callback" {
			state := r.URL.Query().Get("state")
			parts := strings.Split(state, "|")
			if len(parts) != 2 {
				respondWithError(ErrorContext{
					w:          w,
					r:          r,
					statusCode: http.StatusInternalServerError,
					message:    "Error",
				})
				return
			}
			correlationId = parts[1]
		} else {
			correlationId = security.MakeUUIDOrDie()
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), correlationIdKey, correlationId)))
	})
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationId := getCorrelationId(r)
		log.Printf("[%s] - %s\n", correlationId, r.URL)
		next.ServeHTTP(w, r)
	})
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	respondWithError(ErrorContext{
		w:          w,
		r:          r,
		statusCode: http.StatusInternalServerError,
		message:    "Testing error response",
		err:        errors.New("Error object!"),
	})
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, appSessionName)
	if err != nil {
		respondWithError(ErrorContext{
			w:          w,
			r:          r,
			statusCode: http.StatusInternalServerError,
			message:    "Could not retrieve session",
			err:        err,
		})
		return
	}

	if session.Values[authKey] != nil {
		tok := tokenMap[session.Values[authKey].(string)]
		client := conf.Client(ctx, tok)
		response, err := client.Get("https://api.github.com/user")
		if err != nil {
			log.Printf("[%s] Failed to query https://api.github.com/user due to [%s]\n", getCorrelationId(r), err.Error())
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

	state := query.Get("state")
	parts := strings.Split(state, "|")
	mac := parts[0]
	nonce := macMap[mac]
	if nonce == nil || security.ValidMAC(nonce, []byte(mac), []byte(appSecret)) {
		respondWithError(ErrorContext{
			w:          w,
			r:          r,
			statusCode: http.StatusInternalServerError,
			message:    "Validation failed",
			err:        errors.New("Validation failed"),
		})
	}

	code := query.Get("code")
	if len(code) < 1 {
		respondWithError(ErrorContext{
			w:          w,
			r:          r,
			statusCode: http.StatusInternalServerError,
			message:    "Invalid response from identity provider",
			err:        errors.New("Invalid response from identity provider"),
		})
		return
	}
	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		respondWithError(ErrorContext{
			w:          w,
			r:          r,
			statusCode: http.StatusInternalServerError,
			message:    "Could not exchange the authorization code for an acess token",
			err:        err,
		})
		return
	}

	session, err := store.Get(r, appSessionName)
	if err != nil {
		respondWithError(ErrorContext{
			w:          w,
			r:          r,
			statusCode: http.StatusInternalServerError,
			message:    "Could not retrieve session",
			err:        err,
		})
		return
	}

	randStr, err := security.GenerateRandomString(32)
	if err != nil {
		respondWithError(ErrorContext{
			w:          w,
			r:          r,
			statusCode: http.StatusInternalServerError,
			message:    "Could not make random string",
			err:        err,
		})
		return
	}

	tokenMap[randStr] = tok
	session.Values[authKey] = randStr
	err = session.Save(r, w)
	if err != nil {
		respondWithError(ErrorContext{
			w:          w,
			r:          r,
			statusCode: http.StatusInternalServerError,
			message:    "Could not save session",
			err:        err,
		})
		return
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// Writes to and closes `w`, so it cannot be altered elsewhere.
func loginAtIdP(conf *oauth2.Config, w http.ResponseWriter, r *http.Request) {
	nonce, err := security.GenerateRandomBytes(64)
	if err != nil {
		respondWithError(ErrorContext{
			w:          w,
			r:          r,
			statusCode: http.StatusInternalServerError,
			message:    "Could not generate HMAC",
			err:        err,
		})
		return
	}
	mac := security.ConstructMAC(nonce, []byte(appSecret))
	state := fmt.Sprintf("%s|%s", mac, getCorrelationId(r))
	macMap[mac] = nonce
	authCodeUrl := conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, authCodeUrl, http.StatusTemporaryRedirect)
}

func getCorrelationId(r *http.Request) string {
	correlationId := r.Context().Value(correlationIdKey)
	if correlationId == nil {
		return "00000000-0000-0000-0000-000000000000"
	} else {
		return correlationId.(string)
	}
}

type ErrorContext struct {
	w          http.ResponseWriter
	r          *http.Request
	statusCode int
	message    string
	err        error
}

func respondWithError(ectx ErrorContext) {
	correlationId := getCorrelationId(ectx.r)
	// if isDevelopment {
	msgOut := ""
	if ectx.err != nil {
		msgOut := fmt.Sprintf("[%s] %d %s - ERROR: %s", correlationId, ectx.statusCode, ectx.message, ectx.err.Error())
	} else {
		msgOut := fmt.Sprintf("[%s] %d %s", correlationId, ectx.statusCode, ectx.message)
	}
	log.Println(msgOut)
	http.Error(ectx.w, msgOut, ectx.statusCode)
	// } else {...}
}
