package main

import (
	"encoding/json"
	"fmt"
	"github.com/strava/go.strava"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
)

var PORT int = 8080
var client *strava.Client
var athlete strava.AthleteDetailed
var templates = template.Must(template.ParseFiles("input.html", "results.html"))

type SegmentInfo struct {
	Name        string
	PRTime      int
	ElapsedTime int
	Percentage  int
}

type AppConfig struct {
	ClientID     int
	ClientSecret string
}

func main() {
	configFile := "appconfig.json"
	configData, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Printf("Can't open configuration file: %s\n", configFile)
		os.Exit(1)
	}

	var config AppConfig
	err = json.Unmarshal(configData, &config)
	if err != nil {
		fmt.Println("Can't parse configuration file.")
		os.Exit(1)
	}

	strava.ClientId = config.ClientID
	strava.ClientSecret = config.ClientSecret

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/results/", resultsHandler)

	cmd := "open"
	if runtime.GOOS == "windows" {
		cmd = "explorer"
	}
	err = exec.Command(cmd, fmt.Sprintf("http://localhost:%d", PORT)).Start()
	if err != nil {
		fmt.Printf("Please visit http://localhost:%d\n", PORT)
	}
	http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	checkAuth(w, r)
	renderTemplate("input.html", w, nil)
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

	data := make([]SegmentInfo, len(detail.SegmentEfforts))

	for i, effort := range detail.SegmentEfforts {
		elapsedTime := effort.ElapsedTime
		segment, err := segmentService.Get(int(effort.Segment.Id)).Do()
		if err != nil {
			panic(err)
		}
		prTime := segment.PRTime

		data[i].Name = effort.Name
		data[i].PRTime = prTime
		data[i].ElapsedTime = elapsedTime
		data[i].Percentage = int(float32(elapsedTime) / float32(prTime) * 100)
	}

	renderTemplate("results.html", w, data)
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
		http.Redirect(w, r, strava.OAuthAuthorizationURL("", strava.Permissions.Public, false), http.StatusFound)
	}
}

func authSuccess(auth *strava.AuthorizationResponse, w http.ResponseWriter, r *http.Request) {
	client = strava.NewClient(auth.AccessToken)
	athlete = auth.Athlete
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
