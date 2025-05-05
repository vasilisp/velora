package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/extra"
	"github.com/vasilisp/lingograph/openai"
	"github.com/vasilisp/lingograph/store"
	"github.com/vasilisp/velora/internal/db"
	"github.com/vasilisp/velora/internal/fitness"
	"github.com/vasilisp/velora/internal/plan"
	"github.com/vasilisp/velora/internal/template"
	"github.com/vasilisp/velora/internal/util"
)

func addActivityCallback(dbh *sql.DB) func(activity db.ActivityUnsafe, r store.Store) (bool, error) {
	return func(activity db.ActivityUnsafe, r store.Store) (bool, error) {
		activitySafe, err := activity.ToActivity()
		if err != nil {
			return false, fmt.Errorf("malformed activity: %v\n", err)
		}

		activityJSON, err := json.MarshalIndent(activity, "", "  ")
		if err != nil {
			util.Fatalf("error marshalling activity to JSON: %v\n", err)
		}
		fmt.Printf("read activity:\n\n%s\n\n", activityJSON)
		fmt.Printf("does it look correct? (y/n) ")

		var answer string
		_, err = fmt.Scanln(&answer)
		if err != nil {
			util.Fatalf("error reading answer: %v\n", err)
		}

		switch strings.ToLower(answer) {
		case "y", "yes":
			db.InsertActivity(dbh, activitySafe)
			return true, nil
		default:
			return false, nil
		}
	}
}

func addActivity(dbh *sql.DB, args []string) {
	util.Assert(dbh != nil, "addActivity nil dbh")
	util.Assert(len(args) == 1, "Usage: velora add <description>")

	templates := template.MakeParsed([]string{"header", "add"})

	userPrompt := strings.Join(args, " ")

	systemPrompt, err := templates.Execute("add", nil)
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	client := openai.NewClient(openai.APIKeyFromEnv())

	actor := openai.NewActor(client, openai.GPT41Mini, systemPrompt, nil)
	openai.AddFunction(actor, "add_activity", "Add an activity to the database", addActivityCallback(dbh))

	pipeline := lingograph.Chain(
		lingograph.UserPrompt("Today is "+time.Now().Format("2006-01-02"), false),
		lingograph.UserPrompt(userPrompt, false),
		actor.Pipeline(nil, false, 3),
	)

	chat := lingograph.NewChat()

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

func askAI(dbh *sql.DB, userPrompt string, interactive bool) {
	util.Assert(dbh != nil, "askAI nil dbh")
	util.Assert(userPrompt != "" || interactive, "askAI empty userPrompt and interactive is false")

	templates := template.MakeParsed([]string{"header", "ask", "spec_input"})

	systemPrompt, err := templates.Execute("ask", nil)
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	fitnessData, err := fitnessData(dbh)
	if err != nil {
		util.Fatalf("error getting fitness data: %v\n", err)
	}

	client := openai.NewClient(openai.APIKeyFromEnv())
	actor := openai.NewActor(client, openai.GPT41, systemPrompt, nil)

	pipeline := lingograph.Chain(
		lingograph.UserPrompt(fitnessData, false),
	)

	if userPrompt != "" {
		pipeline = lingograph.Chain(pipeline, actor.Pipeline(extra.Echoln(os.Stdout, ""), !interactive, 3))
	}

	if interactive {
		pipeline = lingograph.Chain(pipeline, plan.InteractivePipeline(actor.LingographActor()))
	}

	chat := lingograph.NewChat()

	err = pipeline.Execute(chat)
	if err != nil {
		util.Fatalf("error getting response: %v\n", err)
	}
}

func planWorkouts(dbh *sql.DB, singleStep bool, interactive bool) {
	fitness := fitness.Read(dbh)
	planner := plan.NewPlanner(openai.APIKeyFromEnv(), fitness)

	if singleStep {
		planner.SingleStep(interactive)
	} else {
		planner.MultiStep(interactive)
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
	case "recent":
		showLastActivities(dbh)
	case "plan":
		args := os.Args[2:]
		singleStep := false
		interactive := false
		for _, arg := range args {
			switch arg {
			case "--single-step":
				singleStep = true
			case "--interactive":
				interactive = true
			default:
				util.Fatalf("unknown plan flag: %s\n", arg)
			}
		}
		planWorkouts(dbh, singleStep, interactive)
	case "ask":
		interactive := false
		args := os.Args[2:]
		if len(os.Args) <= 2 {
			askAI(dbh, "", true)
			return
		}
		if os.Args[2] == "--interactive" {
			interactive = true
			args = os.Args[3:]
		} else {
			interactive = false
			args = os.Args[2:]
		}
		askAI(dbh, strings.Join(args, " "), interactive)
	default:
		util.Fatalf("unknown command\n")
	}
}
