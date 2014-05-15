package main

import (
	"fmt"
	"github.com/strava/go.strava"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"strconv"
)

var PORT int = 8080
var client *strava.Client
var athlete strava.AthleteDetailed
var templates = template.Must(template.ParseFiles("input.html"))

func main() {
	strava.ClientId = 734
	strava.ClientSecret = "d199bb61472903a73a7b6c4f70b3cc789b3bb3f9"

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/results/", resultsHandler)

	err := exec.Command("explorer", fmt.Sprintf("http://localhost:%d", PORT)).Start()
	if err != nil {
		fmt.Printf("Please visit http://localhost:%d\n", PORT)
	}
	http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	checkAuth(w, r)
	renderTemplate("input.html", w)
}

func resultsHandler(w http.ResponseWriter, r *http.Request) {
	checkAuth(w, r)

	activitiesService := strava.NewActivitiesService(client)
	segmentService := strava.NewSegmentsService(client)

	activityId, err := strconv.ParseInt(r.FormValue("activity_id"), 0, 64)
	if err != nil {
		// Retrieve last activity
		athletesService := strava.NewAthletesService(client)
		activities, err := athletesService.ListActivities(athlete.Id).Page(1).PerPage(1).Do()
		if err != nil {
			panic(err)
		}
		activityId = activities[0].Id
	}

	detail, err := activitiesService.Get(activityId).IncludeAllEfforts().Do()
	if err != nil {
		panic(err)
	}

	for _, effort := range detail.SegmentEfforts {
		elapsedTime := effort.ElapsedTime
		segment, err := segmentService.Get(int(effort.Segment.Id)).Do()
		if err != nil {
			panic(err)
		}
		prTime := segment.PRTime
		fmt.Println(prTime, elapsedTime)
	}
}

func checkAuth(w http.ResponseWriter, r *http.Request) {
	if client == nil {
		strava.OAuthCallbackURL = fmt.Sprintf("http://localhost:%d/auth/", PORT)
		path, err := strava.OAuthCallbackPath()
		if err != nil {
			fmt.Printf("Can't set authorization callback URL.\n%s\n", err)
			os.Exit(1)
		}
		http.HandleFunc(path, strava.OAuthCallbackHandler(authSuccess, authFailure))
		http.Redirect(w, r, strava.OAuthAuthorizationURL("", strava.Permissions.Public, true), http.StatusFound)
	}
}

func authSuccess(auth *strava.AuthorizationResponse, w http.ResponseWriter, r *http.Request) {
	client = strava.NewClient(auth.AccessToken)
	athlete = auth.Athlete
	http.Redirect(w, r, "/", http.StatusFound)
}

func authFailure(err error, w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>Authorization Unsuccessful</h1><div>Why? %s</div>", err)
}

func renderTemplate(filename string, w http.ResponseWriter) {
	err := templates.ExecuteTemplate(w, filename, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
