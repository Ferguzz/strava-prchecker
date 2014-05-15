package main

import (
	"fmt"
	"github.com/strava/go.strava"
	"net/http"
	// "os/exec"
	"os"
)

func main() {
	PORT := 8080

	strava.ClientId = 734
	strava.ClientSecret = "d199bb61472903a73a7b6c4f70b3cc789b3bb3f9"
	strava.OAuthCallbackURL = fmt.Sprintf("http://localhost:%d/exchange", PORT)

	path, err := strava.OAuthCallbackPath()
	if err != nil {
		fmt.Printf("Can't set authorization callback URL.\n%s\n", err)
		os.Exit(1)
	}

	// The exec command doesn't work at the moment.
	// authURL := strava.OAuthAuthorizationURL("", strava.Permissions.Public, true)
	// fmt.Println(fmt.Sprintf("\"%s\"", authURL))
	// err = exec.Command("explorer", fmt.Sprintf("\"%s\"", authURL)).Start()
	// if err != nil {
	http.HandleFunc("/", indexHandler)
	fmt.Println("Please visit http://localhost:8080 for authorization.")
	// }

	http.HandleFunc(path, strava.OAuthCallbackHandler(oAuthSuccess, oAuthFailure))
	http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, strava.OAuthAuthorizationURL("", strava.Permissions.Public, true), http.StatusFound)
}

func oAuthSuccess(auth *strava.AuthorizationResponse, w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>Authorization Successful!</h1>")

	client := strava.NewClient(auth.AccessToken)
	athletesService := strava.NewAthletesService(client)
	activitiesService := strava.NewActivitiesService(client)
	segmentService := strava.NewSegmentsService(client)

	athleteId := auth.Athlete.Id
	activities, err := athletesService.ListActivities(athleteId).Page(1).PerPage(1).Do()
	if err != nil {
		panic(err)
	}

	detail, err := activitiesService.Get(activities[0].Id).IncludeAllEfforts().Do()
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

func oAuthFailure(err error, w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>Authorization Unsuccessful</h1><div>Why? %s</div>", err)
}
