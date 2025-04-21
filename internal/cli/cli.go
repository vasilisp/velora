package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/openai"
	"github.com/vasilisp/velora/internal/db"
	"github.com/vasilisp/velora/internal/fitness"
	"github.com/vasilisp/velora/internal/plan"
	"github.com/vasilisp/velora/internal/template"
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

func readActivityAI(dbh *sql.DB, response string) {
	util.Assert(dbh != nil, "readActivityAI nil dbh")

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

func addActivityAI(dbh *sql.DB, args []string) {
	util.Assert(dbh != nil, "addActivityAI nil dbh")
	util.Assert(len(args) == 1, "Usage: velora addai <description>")

	templates := template.MakeParsed([]string{"header", "add"})

	userPrompt := strings.Join(args, " ")

	systemPrompt, err := templates.Execute("add", nil)
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	echo := func(message lingograph.Message) {
		readActivityAI(dbh, message.Content)
		os.Exit(0)
	}

	client := openai.NewClient(openai.APIKeyFromEnv())
	actor := openai.NewActor(client, openai.GPT41Mini, systemPrompt)

	pipeline := lingograph.Chain(
		lingograph.UserPrompt("Today is "+time.Now().Format("2006-01-02"), false),
		lingograph.UserPrompt(userPrompt, false),
		actor.Pipeline(echo, false, 3),
	)

	chat := lingograph.NewSliceChat()

	err = pipeline.Execute(chat)
	if err != nil {
		util.Fatalf("error getting activity: %v\n", err)
	}
}

func showLastActivities(dbh *sql.DB) {
	activities, err := db.LastActivities(dbh, 10)
	if err != nil {
		util.Fatalf("error getting last activities: %v\n", err)
	}

	for i, activity := range activities {
		fmt.Println(activity.Show())
		if i < len(activities)-1 {
			fmt.Println()
		}
	}
}

func fitnessData(dbh *sql.DB) (string, error) {
	util.Assert(dbh != nil, "fitnessData nil dbh")

	fitnessData := fitness.Read(dbh)

	fitnessBytes, err := json.MarshalIndent(fitnessData, "", "  ")
	if err != nil {
		util.Fatalf("error marshalling fitness data: %v\n", err)
	}

	return string(fitnessBytes), nil
}

func askAI(dbh *sql.DB, mode string, userPrompt string) {
	util.Assert(dbh != nil, "askAI nil dbh")

	templates := template.MakeParsed([]string{"header", "ask", "spec_input"})

	systemPrompt, err := templates.Execute(mode, nil)
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	fitnessData, err := fitnessData(dbh)
	if err != nil {
		util.Fatalf("error getting fitness data: %v\n", err)
	}

	echo := func(message lingograph.Message) {
		fmt.Println(util.SanitizeOutput(message.Content, false))
	}

	client := openai.NewClient(openai.APIKeyFromEnv())
	actor := openai.NewActor(client, openai.GPT41, systemPrompt)

	pipeline := lingograph.Chain(
		lingograph.UserPrompt(fitnessData, false),
		lingograph.UserPrompt(userPrompt, false),
		actor.Pipeline(echo, false, 3),
	)

	chat := lingograph.NewSliceChat()

	err = pipeline.Execute(chat)
	if err != nil {
		util.Fatalf("error getting response: %v\n", err)
	}
}

func tuneAI() {
	templates := template.MakeParsed([]string{"header", "spec_input", "spec_output", "tune"})

	userPrompt, err := templates.Execute("tune", nil)
	if err != nil {
		util.Fatalf("error getting user prompt: %v\n", err)
	}

	systemPrompt, err := templates.Execute("tune", nil)
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	echo := func(message lingograph.Message) {
		fmt.Println(util.SanitizeOutput(message.Content, false))
	}

	client := openai.NewClient(openai.APIKeyFromEnv())
	actor := openai.NewActor(client, openai.GPT41, systemPrompt)

	pipeline := lingograph.Chain(
		lingograph.UserPrompt(userPrompt, false),
		actor.Pipeline(echo, false, 3),
	)

	chat := lingograph.NewSliceChat()

	err = pipeline.Execute(chat)
	if err != nil {
		util.Fatalf("error getting workout recommendation: %v\n", err)
	}
}

func planWorkouts(dbh *sql.DB, multiStep bool) {
	fitness := fitness.Read(dbh)
	planner := plan.NewPlanner(openai.APIKeyFromEnv(), fitness)

	if multiStep {
		planner.MultiStep()
	} else {
		planner.SingleStep()
	}
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
	case "plan":
		planWorkouts(dbh, len(os.Args) > 2 && os.Args[2] == "--multi-step")
	case "ask":
		askAI(dbh, "ask", strings.Join(os.Args[2:], " "))
	case "tune":
		tuneAI()
	default:
		util.Fatalf("unknown command\n")
	}
}
