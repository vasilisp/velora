package cli

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vasilisp/velora/internal/data"
	"github.com/vasilisp/velora/internal/db"
	"github.com/vasilisp/velora/internal/openai"
	"github.com/vasilisp/velora/internal/util"
)

func addActivity(dbh *sql.DB, args []string) {
	if len(args) < 3 {
		util.Fatalf("Usage: velora add <sport> <duration> <distance>")
	}

	sportStr := args[0]
	sport, err := db.SportFromString(sportStr)
	if err != nil {
		util.Fatalf("Invalid sport: %s\n", sportStr)
	}

	duration, err := strconv.Atoi(args[1])
	if err != nil {
		util.Fatalf("invalid duration: %s\n", args[1])
	}

	distance, err := strconv.Atoi(args[2])
	if err != nil {
		util.Fatalf("invalid distance: %s\n", args[2])
	}

	verticalGain, err := strconv.Atoi(args[3])
	if err != nil {
		util.Fatalf("invalid vertical gain: %s\n", args[3])
	}

	notes := ""
	if len(args) > 4 {
		notes = strings.Join(args[4:], " ")
	}

	activity, err := db.NewActivity(time.Now(), duration, duration, distance, sport, verticalGain, notes)
	if err != nil {
		util.Fatalf("error creating activity: %v\n", err)
	}

	util.Assert(activity != nil, "add nil activity")

	if err := db.InsertActivity(dbh, *activity); err != nil {
		util.Fatalf("error inserting activity: %v\n", err)
	}
}
func formatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func formatDistance(meters int) string {
	if meters >= 1000 {
		return fmt.Sprintf("%.1fkm", float64(meters)/1000)
	}
	return fmt.Sprintf("%dm", meters)
}

func showLastActivities(dbh *sql.DB) {
	activities, err := db.LastActivities(dbh, 10)
	if err != nil {
		util.Fatalf("error getting last activities: %v\n", err)
	}

	for i, activity := range activities {
		fmt.Printf("Date: %s\n", activity.Timestamp.Format("Jan 2, 15:04"))
		fmt.Printf("Sport: %s\n", activity.Sport)
		fmt.Printf("Time: %s\n", formatDuration(activity.Duration))
		fmt.Printf("Distance: %s\n", formatDistance(activity.Distance))
		fmt.Printf("Vertical Gain: %dm\n", activity.VerticalGain)
		fmt.Printf("Notes: %s\n", activity.Notes)
		if i < len(activities)-1 {
			fmt.Print("\n")
		}
	}
}

func systemPromptNext() (string, error) {
	fsys, err := data.PromptFS()
	if err != nil {
		util.Fatalf("error getting prompt FS: %v\n", err)
	}

	t, err := template.ParseFS(fsys, "header", "next")
	if err != nil {
		util.Fatalf("error parsing template: %v\n", err)
	}

	var systemPrompt bytes.Buffer
	if err := t.ExecuteTemplate(&systemPrompt, "next", nil); err != nil {
		util.Fatalf("error executing template: %v\n", err)
	}

	return systemPrompt.String(), nil
}

func userPromptNext(dbh *sql.DB) (string, error) {
	util.Assert(dbh != nil, "userPromptNext nil dbh")

	activities, err := db.LastActivities(dbh, 10)
	if err != nil {
		util.Fatalf("error getting last activities: %v\n", err)
	}

	var activityStrings []string
	for _, activity := range activities {
		verticalGainStr := ""
		if activity.VerticalGain > 0 {
			verticalGainStr = fmt.Sprintf(" with %dm elevation gain", activity.VerticalGain)
		}
		activityStr := fmt.Sprintf("%s: %s for %s covering %s%s",
			activity.Timestamp.Format("Jan 2"),
			activity.Sport,
			formatDuration(activity.Duration),
			formatDistance(activity.Distance),
			verticalGainStr)
		activityStrings = append(activityStrings, activityStr)
	}

	prefsPath := filepath.Join(os.Getenv("HOME"), ".velora", "velora.prefs")
	prefsContent := ""
	if prefs, err := os.ReadFile(prefsPath); err == nil {
		prefsContent = fmt.Sprintf("My workout preferences:\n%s\n", string(prefs))
	}

	userPrompt := fmt.Sprintf("%sHere are my recent activities:\n%s\n\nWhat should I do for my next workout?",
		prefsContent,
		strings.Join(activityStrings, "\n"))

	return userPrompt, nil
}

func nextWorkout(dbh *sql.DB) {
	util.Assert(dbh != nil, "nextWorkout nil dbh")

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		util.Fatalf("OPENAI_API_KEY environment variable not set\n")
	}

	client := openai.NewClient(apiKey)

	userPrompt, err := userPromptNext(dbh)
	if err != nil {
		util.Fatalf("error getting user prompt: %v\n", err)
	}

	systemPrompt, err := systemPromptNext()
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	response, err := client.AskGPT(systemPrompt, userPrompt)
	if err != nil {
		util.Fatalf("error getting workout recommendation: %v\n", err)
	}

	fmt.Println(response)
}

func Main() {
	dbh, err := db.Init()
	if err != nil {
		util.Fatalf("Error initializing database: %v\n", err)
	}
	defer dbh.Close()

	if len(os.Args) <= 1 {
		showLastActivities(dbh)
		return
	}

	switch os.Args[1] {
	case "add":
		addActivity(dbh, os.Args[2:])
	case "recent":
		showLastActivities(dbh)
	case "next":
		nextWorkout(dbh)
	}
}
