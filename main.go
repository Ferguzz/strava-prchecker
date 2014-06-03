package prchecker

import (
	"appengine"
	"appengine/urlfetch"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/strava/go.strava"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"
)

var authenticator *strava.OAuthAuthenticator
var store = sessions.NewCookieStore(securecookie.GenerateRandomKey(64))
var sessionName = "prchecker"
var templates = template.Must(template.ParseFiles("html/input.html", "html/results.html"))

type SegmentInfo struct {
	Name        string
	PRTime      int
	ElapsedTime int
	Percentage  int
}

type AppConfig struct {
	Domain       string
	ClientID     int
	ClientSecret string
}

func init() {
	configFile := "appconfig.json"
	configData, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatal("Can't open configuration file: %s\n", configFile)
	}

	var config AppConfig
	err = json.Unmarshal(configData, &config)
	if err != nil {
		log.Fatal("Can't parse configuration file.\n")
	}

	strava.ClientId = config.ClientID
	strava.ClientSecret = config.ClientSecret

	authenticator = &strava.OAuthAuthenticator{
		CallbackURL: fmt.Sprintf("%s/auth/", config.Domain),
		RequestClientGenerator: func(r *http.Request) *http.Client {
			return urlfetch.Client(appengine.NewContext(r))
		},
	}

	path, err := authenticator.CallbackPath()
	if err != nil {
		log.Fatal("Can't set authorization callback URL: %s\n", err)
	}

	r := mux.NewRouter()
	r.HandleFunc(path, authenticator.HandlerFunc(authSuccess, authFailure))
	r.HandleFunc("/", homeHandler)
	r.HandleFunc("/results/", resultsHandler)
	http.Handle("/", r)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, sessionName)
	if !checkAuth(session) {
		authHandler(w, r)
	} else {
		renderTemplate("input.html", w, nil)
	}
}

func resultsHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, sessionName)
	if !checkAuth(session) {
		authHandler(w, r)
	} else {
		c := appengine.NewContext(r)
		tr := urlfetch.Transport{
			Context:                       c,
			Deadline:                      60 * time.Second,
			AllowInvalidServerCertificate: false,
		}

		accessToken, _ := session.Values["accessToken"].(string)
		athleteId, _ := session.Values["athleteId"].(int64)
		client := strava.NewClient(accessToken, &http.Client{Transport: &tr})

		activityId, err := strconv.ParseInt(r.FormValue("activity_id"), 0, 64)
		if err != nil {
			// Retrieve last activity
			athletesService := strava.NewAthletesService(client)
			activities, err := athletesService.ListActivities(athleteId).Page(1).PerPage(1).Do()
			if err != nil {
				http.Error(w, fmt.Sprintf("Can't get requested activity: %s", err), http.StatusInternalServerError)
			}
			activityId = activities[0].Id
		}

		activitiesService := strava.NewActivitiesService(client)
		detail, err := activitiesService.Get(activityId).IncludeAllEfforts().Do()
		if err != nil {
			http.Error(w, fmt.Sprintf("Can't get segment efforts for requested activity: %s", err), http.StatusInternalServerError)
		}

		data := make([]SegmentInfo, len(detail.SegmentEfforts))
		segmentService := strava.NewSegmentsService(client)

		for i, effort := range detail.SegmentEfforts {
			elapsedTime := effort.ElapsedTime
			segment, err := segmentService.Get(int(effort.Segment.Id)).Do()
			if err != nil {
				http.Error(w, fmt.Sprintf("Can't get segment detail for requested activity: %s", err), http.StatusInternalServerError)
			}
			prTime := segment.PRTime

			data[i].Name = effort.Name
			data[i].PRTime = prTime
			data[i].ElapsedTime = elapsedTime
			data[i].Percentage = int(float32(elapsedTime) / float32(prTime) * 100)
		}
		renderTemplate("results.html", w, data)
	}
}

func checkAuth(session *sessions.Session) bool {
	if v, ok := session.Values["authenticated"]; ok && v == true {
		return true
	} else {
		return false
	}
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, authenticator.AuthorizationURL("prchecker", strava.Permissions.Public, false), http.StatusFound)
}

func authSuccess(auth *strava.AuthorizationResponse, w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, sessionName)
	session.Values["authenticated"] = true
	session.Values["accessToken"] = auth.AccessToken
	session.Values["athleteId"] = auth.Athlete.Id
	if err := session.Save(r, w); err != nil {
		http.Error(w, fmt.Sprintf("Couldn't save session: %s\n", err), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func authFailure(err error, w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>Authorization Unsuccessful</h1><div>%s</div>", err)
}

func renderTemplate(filename string, w http.ResponseWriter, data interface{}) {
	err := templates.ExecuteTemplate(w, filename, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
