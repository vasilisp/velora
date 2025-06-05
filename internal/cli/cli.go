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
	"github.com/vasilisp/lingograph/extra"
	"github.com/vasilisp/lingograph/openai"
	"github.com/vasilisp/lingograph/store"
	"github.com/vasilisp/velora/internal/db"
	"github.com/vasilisp/velora/internal/fitness"
	"github.com/vasilisp/velora/internal/plan"
	"github.com/vasilisp/velora/internal/template"
	"github.com/vasilisp/velora/internal/util"
)

func addActivityCallback(dbh *sql.DB, didAdd store.Var[bool]) func(activity db.ActivityUnsafe, r store.Store) (db.ActivityUnsafe, error) {
	return func(activity db.ActivityUnsafe, r store.Store) (db.ActivityUnsafe, error) {
		activitySafe, err := activity.ToActivity()
		if err != nil {
			return activity, fmt.Errorf("malformed activity: %v\n", err)
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
			store.Set(r, didAdd, true)
			return activity, nil
		default:
			return activity, nil
		}
	}
}

func writeSkeletonCallback(skeleton plan.Skeleton, r store.Store) (bool, error) {
	skeletonString, err := json.MarshalIndent(skeleton, "", "  ")
	if err != nil {
		return false, err
	}

	fmt.Printf("Skeleton:\n\n%s\n\n", skeletonString)
	fmt.Printf("Does it look correct? (y/n) ")

	err = plan.WriteSkeleton(&skeleton)
	if err != nil {
		return false, err
	}

	return true, nil
}

func analyzeAddedActivity(dbh *sql.DB, client openai.Client, templates template.Parsed, didAdd store.Var[bool]) lingograph.Pipeline {
	util.Assert(dbh != nil, "analyzeAddedActivity nil dbh")

	systemPromptComment, err := templates.Execute("header", nil)
	if err != nil {
		util.Fatalf("error getting header template: %v\n", err)
	}

	templateComment, err := templates.Execute("add_comment", nil)
	if err != nil {
		util.Fatalf("error getting add_comment template: %v\n", err)
	}

	actorComment := openai.NewActor(client, openai.GPT41, systemPromptComment, nil)

	fitnessData, err := fitnessData(dbh)
	if err != nil {
		util.Fatalf("error getting fitness data: %v\n", err)
	}

	return lingograph.If(
		func(r store.StoreRO) bool {
			didAdd, found := store.GetRO(r, didAdd)
			return found && didAdd
		},
		lingograph.Chain(
			lingograph.UserPrompt(templateComment, false),
			lingograph.UserPrompt(fitnessData, false),
			actorComment.Pipeline(extra.Echoln(os.Stdout, ""), false, 3),
		),
		lingograph.UserPrompt("The activity was not added to the database.", false),
	)
}

func addActivity(dbh *sql.DB, args []string, analyze bool) {
	util.Assert(dbh != nil, "addActivity nil dbh")
	util.Assert(len(args) == 1, "Usage: velora add <description>")

	templates := template.MakeParsed([]string{"header", "add", "add_comment", "spec_input"})

	userPrompt := strings.Join(args, " ")

	systemPrompt, err := templates.Execute("add", nil)
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	client := openai.NewClient(openai.APIKeyFromEnv())

	didAdd := store.FreshVar[bool]()
	actor := openai.NewActor(client, openai.GPT41Mini, systemPrompt, nil)
	openai.AddFunction(actor, "add_activity", "Add an activity to the database", addActivityCallback(dbh, didAdd))

	timezone, _ := time.Now().Zone()
	pipeline := lingograph.Chain(
		lingograph.UserPrompt(fmt.Sprintf("Today is %s\n\n. The user is in the %s timezone.", time.Now().Format("2006-01-02"), timezone), false),
		lingograph.UserPrompt(userPrompt, false),
		actor.Pipeline(nil, true, 3),
	)

	if analyze {
		pipeline = lingograph.Chain(
			pipeline,
			analyzeAddedActivity(dbh, client, templates, didAdd),
		)
	}

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
		activity.OutputTo(os.Stdout)
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

	args := map[string]any{
		"order":       "first",
		"json_schema": fitness.JSONSchema(),
	}

	systemPrompt, err := templates.Execute("ask", args)
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	fitnessData, err := fitnessData(dbh)
	if err != nil {
		util.Fatalf("error getting fitness data: %v\n", err)
	}

	client := openai.NewClient(openai.APIKeyFromEnv())

	actor := openai.NewActor(client, openai.GPT41, systemPrompt, nil)
	openai.AddFunction(actor, "write_skeleton", "Write a skeleton to the database", writeSkeletonCallback)

	pipeline := lingograph.Chain(
		lingograph.UserPrompt(fitnessData, false),
	)

	if userPrompt != "" {
		pipeline = lingograph.Chain(
			pipeline,
			lingograph.UserPrompt(userPrompt, false),
			actor.Pipeline(extra.Echoln(os.Stdout, ""), !interactive, 3),
		)
	}

	if interactive {
		pipeline = lingograph.Chain(pipeline, plan.InteractivePipeline(actor))
	}

	chat := lingograph.NewChat()

	err = pipeline.Execute(chat)
	if err != nil {
		util.Fatalf("error getting response: %v\n", err)
	}
}

func planWorkouts(dbh *sql.DB, singleStep bool, interactive bool, numDays int) {
	fitness := fitness.Read(dbh)
	planner := plan.NewPlanner(openai.APIKeyFromEnv(), fitness)

	if singleStep {
		planner.SingleStep(interactive, numDays)
	} else {
		planner.MultiStep(interactive, numDays)
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
		addActivity(dbh, os.Args[2:], true)
	case "recent":
		showLastActivities(dbh)
	case "plan":
		args := os.Args[2:]
		singleStep := false
		interactive := false
		numDays := 3

		for i := 0; i < len(args); i++ {
			arg := args[i]
			switch arg {
			case "--single-step":
				singleStep = true
			case "--interactive":
				interactive = true
			case "--num-days":
				if i+1 >= len(args) {
					util.Fatalf("--num-days requires a value\n")
				}

				var err error
				numDays, err = strconv.Atoi(args[i+1])
				if err != nil {
					util.Fatalf("invalid value for --num-days: %v\n", err)
				}

				if numDays <= 0 {
					util.Fatalf("--num-days must be positive\n")
				}
				i++ // skip the next argument since we've consumed it
			default:
				util.Fatalf("unknown plan flag: %s\n", arg)
			}
		}
		planWorkouts(dbh, singleStep, interactive, numDays)
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
