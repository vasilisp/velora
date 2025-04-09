package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/vasilisp/velora/internal/data"
	"github.com/vasilisp/velora/internal/db"
	"github.com/vasilisp/velora/internal/langchain"
	"github.com/vasilisp/velora/internal/profile"
	"github.com/vasilisp/velora/internal/util"
)

func langChainClient() langchain.Client {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		util.Fatalf("OPENAI_API_KEY environment variable not set\n")
	}

	client, err := langchain.NewClient(apiKey)
	if err != nil {
		util.Fatalf("error creating OpenAI client: %v\n", err)
	}
	return client
}

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

	activityUnsafe := db.ActivityUnsafe{
		Time:          time.Now(),
		Duration:      duration,
		DurationTotal: duration,
		Distance:      distance,
		Sport:         sport,
		VerticalGain:  verticalGain,
		Notes:         notes,
	}

	activity, err := activityUnsafe.ToActivity()
	if err != nil {
		util.Fatalf("error creating activity: %v\n", err)
	}

	if err := db.InsertActivity(dbh, activity); err != nil {
		util.Fatalf("error inserting activity: %v\n", err)
	}
}

func systemPromptAdd() (string, error) {
	fsys, err := data.PromptFS()
	if err != nil {
		util.Fatalf("error getting prompt FS: %v\n", err)
	}

	t, err := template.ParseFS(fsys, "header", "add")
	if err != nil {
		util.Fatalf("error parsing template: %v\n", err)
	}

	var systemPrompt bytes.Buffer
	if err := t.ExecuteTemplate(&systemPrompt, "add", nil); err != nil {
		util.Fatalf("error executing template: %v\n", err)
	}

	return systemPrompt.String(), nil
}

func addActivityAI(dbh *sql.DB, args []string) {
	util.Assert(len(args) == 1, "Usage: velora addai <description>")

	client := langChainClient()

	userPrompt := strings.Join(args, " ")

	systemPrompt, err := systemPromptAdd()
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	response, err := client.AskGPT(systemPrompt, []string{"Today is " + time.Now().Format("2006-01-02"), userPrompt})
	if err != nil {
		util.Fatalf("error getting activity: %v\n", err)
	}

	var activityUnsafe db.ActivityUnsafe
	if err := (&activityUnsafe).UnmarshalJSON([]byte(response)); err != nil {
		util.Fatalf("error unmarshalling activity: %v\n", err)
	}

	activity, err := activityUnsafe.ToActivity()
	if err != nil {
		util.Fatalf("error converting activity: %v\n", err)
	}

	fmt.Printf("read activity:\n\n%s\n\ndoes it look correct? (y/n) ", util.SanitizeOutput(response, false))

	var answer string
	_, err = fmt.Scanln(&answer)
	if err != nil {
		util.Fatalf("error reading answer: %v\n", err)
	}
	switch strings.ToLower(answer) {
	case "y", "yes":
		break
	default:
		os.Exit(0)
	}

	if err := db.InsertActivity(dbh, activity); err != nil {
		util.Fatalf("error inserting activity: %v\n", err)
	}
}

func showLastActivities(dbh *sql.DB) {
	activities, err := db.LastActivities(dbh, 10)
	if err != nil {
		util.Fatalf("error getting last activities: %v\n", err)
	}

	for i, activity := range activities {
		println(activity.Show())
		if i < len(activities)-1 {
			println()
		}
	}
}

func executeTemplate(templateName string, allTemplates []string) (string, error) {
	fsys, err := data.PromptFS()
	if err != nil {
		util.Fatalf("error getting prompt FS: %v\n", err)
	}

	t, err := template.ParseFS(fsys, allTemplates...)
	if err != nil {
		util.Fatalf("error parsing template: %v\n", err)
	}

	var systemPrompt bytes.Buffer
	if err := t.ExecuteTemplate(&systemPrompt, templateName, nil); err != nil {
		util.Fatalf("error executing template: %v\n", err)
	}

	return systemPrompt.String(), nil
}

func userPromptData(dbh *sql.DB) ([]string, error) {
	util.Assert(dbh != nil, "userPromptData nil dbh")

	activities, err := db.LastActivities(dbh, 10)
	if err != nil {
		util.Fatalf("error getting last activities: %v\n", err)
	}

	activityMessage, err := json.MarshalIndent(activities, "", "  ")
	if err != nil {
		util.Fatalf("error marshalling activities: %v\n", err)
	}

	prefsPath := filepath.Join(os.Getenv("HOME"), ".velora", "prefs.json")
	prefsBytes := []byte{}
	if prefs, err := os.ReadFile(prefsPath); err == nil {
		var p profile.Profile
		if err := json.Unmarshal(prefs, &p); err != nil {
			util.Fatalf("error unmarshalling prefs: %v\n", err)
		}
		prefsBytes, err = json.MarshalIndent(p, "", "  ")
		if err != nil {
			util.Fatalf("error marshalling prefs: %v\n", err)
		}
	}

	userPrompt := []string{
		string(prefsBytes),
		string(activityMessage),
	}

	return userPrompt, nil
}

func askAI(dbh *sql.DB, mode string, systemPromptTemplates []string, userPromptExtra []string) {
	util.Assert(dbh != nil, "askAI nil dbh")

	client := langChainClient()

	systemPrompt, err := executeTemplate(mode, systemPromptTemplates)
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}
	fmt.Println(systemPrompt)

	userPromptData, err := userPromptData(dbh)
	if err != nil {
		util.Fatalf("error getting user prompt: %v\n", err)
	}
	userPrompt := append(userPromptData, userPromptExtra...)

	response, err := client.AskGPT(systemPrompt, userPrompt)
	if err != nil {
		util.Fatalf("error getting workout recommendation: %v\n", err)
	}

	fmt.Println(util.SanitizeOutput(response, false))
}

func tuneAI() {
	client := langChainClient()

	userPrompt, err := executeTemplate("tune", []string{"tune", "header", "spec_input", "spec_output"})
	if err != nil {
		util.Fatalf("error getting user prompt: %v\n", err)
	}
	fmt.Println(userPrompt)

	response, err := client.AskGPT("", []string{userPrompt})
	if err != nil {
		util.Fatalf("error getting workout recommendation: %v\n", err)
	}

	fmt.Println(util.SanitizeOutput(response, false))
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
	case "addai":
		addActivityAI(dbh, os.Args[2:])
	case "recent":
		showLastActivities(dbh)
	case "next":
		askAI(dbh, "next", []string{"header", "next", "spec_input", "spec_output"}, nil)
	case "ask":
		askAI(dbh, "ask", []string{"header", "ask", "spec_input"}, []string{strings.Join(os.Args[2:], " ")})
	case "tune":
		tuneAI()
	}
}
