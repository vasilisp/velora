package plan

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/extra"
	"github.com/vasilisp/lingograph/openai"
	"github.com/vasilisp/lingograph/store"
	"github.com/vasilisp/velora/internal/db"
	"github.com/vasilisp/velora/internal/fitness"
	"github.com/vasilisp/velora/internal/profile"
	"github.com/vasilisp/velora/internal/template"
	"github.com/vasilisp/velora/internal/util"
)

const moreDeterministic = 0.2

type Planner struct {
	client    openai.Client
	fitness   *fitness.Fitness
	templates template.Parsed
}

type PlanDay struct {
	Date     string       `json:"date" jsonschema_description:"The date of the planned workout in YYYY-MM-DD format"`
	Sport    string       `json:"sport" jsonschema_description:"The type of sport (running, cycling, swimming)"`
	Distance int          `json:"distance" jsonschema_description:"The planned distance in meters"`
	Notes    string       `json:"notes" jsonschema_description:"Additional notes and instructions for the workout, in one line"`
	Segments []db.Segment `json:"segments" jsonschema_description:"The segments of the workout"`
}

type Plan struct {
	Days        []PlanDay `json:"days"`
	Explanation string    `json:"explanation" jsonschema_description:"A short explanation of the choices made, in one paragraph maximum"`
}

func (p Plan) Write(out io.Writer) {
	fmt.Fprintf(out, "Plan:\n\n")
	for _, day := range p.Days {
		fmt.Fprintf(out, "  - Date: %s\n    Sport: %s\n    Distance: %d\n    Notes: %s\n",
			day.Date, day.Sport, day.Distance, day.Notes)
		for _, segment := range day.Segments {
			fmt.Fprintf(out, "      - Repeat: %d\n        Distance: %d\n        Zone: %d\n",
				segment.Repeat, segment.Distance, segment.Zone)
		}
	}
	fmt.Fprintf(out, "\nExplanation: %s\n", p.Explanation)
}

func NewPlanner(apiKey string, fitness *fitness.Fitness) Planner {
	templates := []string{"header", "plan_*", "sched_*", "spec_*"}
	client := openai.NewClient(apiKey)
	return Planner{client: client, fitness: fitness, templates: template.MakeParsed(templates)}
}

func (p Planner) systemPrompt() string {
	context := map[string]any{
		"order":       "first",
		"json_schema": fitness.JSONSchema(),
	}
	systemPromptStr, err := p.templates.Execute("plan_system", context)
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	return systemPromptStr
}

type allowedDisallowedDays struct {
	Allowed    []time.Time
	Disallowed []time.Time
}

type allowedDisallowedDayStrings struct {
	Allowed    []string
	Disallowed []string
}

func nextNDays(fitnessData *fitness.Fitness, sport profile.Sport, numDays int) allowedDisallowedDays {
	skeleton := fitnessData.Skeleton
	sportStr := sport.String()
	today := time.Now()
	startDate := today

	days := allowedDisallowedDays{
		Allowed:    []time.Time{},
		Disallowed: []time.Time{},
	}

	// Check if there's an activity today
	for _, activity := range fitnessData.ActivitiesThisWeek {
		if activity.Time.Format("2006-01-02") == today.Format("2006-01-02") {
			// Activity found today, start planning from tomorrow
			startDate = today.AddDate(0, 0, 1)
			break
		}
	}

	for i := range numDays {
		date := startDate.AddDate(0, 0, i)
		weekday := date.Weekday().String()
		allowed := true
		for _, conflict := range skeleton.Conflicts {
			if conflict.Weekday == weekday && conflict.Sport == sportStr {
				allowed = false
				break
			}
		}

		if allowed {
			days.Allowed = append(days.Allowed, date)
		} else {
			days.Disallowed = append(days.Disallowed, date)
		}
	}

	return days
}

func FormatDates(dates []time.Time) []string {
	formattedDates := make([]string, len(dates))
	for i, date := range dates {
		formattedDates[i] = date.Format("2006-01-02 (Mon)")
	}
	return formattedDates
}

func (p Planner) userPromptOfSport(sport profile.Sport, numDays int) (string, allowedDisallowedDays) {
	days := nextNDays(p.fitness, sport, numDays)

	m := map[string]any{
		"allowed":    FormatDates(days.Allowed),
		"disallowed": FormatDates(days.Disallowed),
		"sport":      sport.String(),
		"numDays":    numDays,
	}

	if len(days.Allowed) == 0 {
		// no allowed days, no need to plan
		return "", days
	}

	var str string
	templateName := fmt.Sprintf("plan_%s", sport)
	if !p.templates.Has(templateName) {
		templateName = "plan_sport"
	}

	str, err := p.templates.Execute(templateName, m)
	if err != nil {
		util.Fatalf("error executing %s template: %v\n", templateName, err)
	}

	return str, days
}

type sportData struct {
	UserPrompt string
	Days       allowedDisallowedDays
	Response   string
}

func (p Planner) templateMultiSportArgs(filterUnavailable bool) map[string]any {
	sports := make([]string, 0, len(p.fitness.Profile.AllSports()))
	sportsCapitalized := make([]string, 0, len(sports))
	days := make(map[string]allowedDisallowedDayStrings)

	for _, sport := range p.fitness.Profile.AllSports() {
		allowedDisallowedDays := nextNDays(p.fitness, sport, 3)

		if filterUnavailable && len(allowedDisallowedDays.Allowed) == 0 {
			continue
		}

		sports = append(sports, sport.String())
		sportsCapitalized = append(sportsCapitalized, util.Capitalize(sport.String()))
		days[sport.String()] = allowedDisallowedDayStrings{
			Allowed:    FormatDates(allowedDisallowedDays.Allowed),
			Disallowed: FormatDates(allowedDisallowedDays.Disallowed),
		}
	}

	return map[string]any{
		"sports":            sports,
		"sportsCapitalized": sportsCapitalized,
		"days":              days,
	}
}

func (p Planner) userPromptCombine(numDays int) string {
	args := p.templateMultiSportArgs(true)
	args["numDays"] = numDays

	str, err := p.templates.Execute("plan_combine", args)
	if err != nil {
		util.Fatalf("error getting combine user prompt: %v\n", err)
	}

	return str
}

func userPromptFitness(fitness *fitness.Fitness) string {
	bytes, err := json.MarshalIndent(fitness, "", "  ")
	if err != nil {
		util.Fatalf("error marshalling fitness data: %v\n", err)
	}

	return string(bytes)
}

func actorOutputPlan(client openai.Client, model openai.ChatModel, systemPrompt string) openai.Actor {
	temperature := moreDeterministic
	actor := openai.NewActor(client, model, systemPrompt, &temperature)

	openai.AddFunction(actor, "output_plan", "Output the plan to the user", func(plan Plan, store store.Store) (string, error) {
		fmt.Println("")
		plan.Write(os.Stdout)
		return "plan received", nil
	})

	return actor
}

const systemPromptSummarize = `
Your task is to summarize the given workout plan and output it to the user.

Only respond with a function call.
`

func (p Planner) singleSport(sport profile.Sport, userPrompt string) {
	temperature := moreDeterministic
	actor := openai.NewActor(p.client, openai.GPT41, p.systemPrompt(), &temperature)
	actorOutputPlan := actorOutputPlan(p.client, openai.GPT41Nano, systemPromptSummarize)

	echo := extra.Echoln(os.Stdout, "")

	pipeline := lingograph.Chain(
		lingograph.UserPrompt(userPromptFitness(p.fitness), false),
		lingograph.UserPrompt(userPrompt, false),
		actor.Pipeline(echo, true, 3),
		actorOutputPlan.Pipeline(echo, false, 3),
	)

	chat := lingograph.NewChat()

	err := pipeline.Execute(chat)

	if err != nil {
		util.Fatalf("error getting %s sport plan: %v\n", sport.String(), err)
	}
}

func InteractivePipeline(actor lingograph.Actor) lingograph.Pipeline {
	return lingograph.While(
		func(store.StoreRO) bool {
			print("> ")

			// dummy; EOF will terminate
			return true
		},
		lingograph.Chain(
			extra.Stdin().Pipeline(nil, false, 0),
			actor.Pipeline(extra.Echoln(os.Stdout, ""), false, 1),
		),
	)
}

func (p Planner) MultiStep(interactive bool, numDays int) {
	sportMap := make(map[profile.Sport]*sportData)

	for _, sport := range p.fitness.Profile.AllSports() {
		userPrompt, days := p.userPromptOfSport(sport, numDays)
		if len(days.Allowed) == 0 {
			continue
		}
		sportMap[sport] = &sportData{
			UserPrompt: userPrompt,
			Days:       days,
			Response:   "",
		}
	}

	switch len(sportMap) {
	case 0:
		fmt.Println("[]")
		return
	case 1:
		for sport, data := range sportMap {
			p.singleSport(sport, data.UserPrompt)
		}
		return
	}

	systemPrompt := p.systemPrompt()
	userPromptFitness := userPromptFitness(p.fitness)

	temperature := moreDeterministic

	actor := openai.NewActor(p.client, openai.GPT41, systemPrompt, &temperature)

	fitnessPrompt := lingograph.UserPrompt(userPromptFitness, false)

	parallelTasks := make([]lingograph.Pipeline, 0, len(sportMap))
	for sport, data := range sportMap {
		parallelTasks = append(parallelTasks, lingograph.Chain(
			fitnessPrompt,
			lingograph.UserPrompt(data.UserPrompt, false),
			actor.Pipeline(
				extra.Echoln(os.Stderr, fmt.Sprintf("%s Draft Plan\n\n", util.Capitalize(sport.String()))),
				false,
				3,
			),
		))
	}

	actorOutputPlan := actorOutputPlan(p.client, openai.GPT41Nano, systemPromptSummarize)

	pipeline := lingograph.Chain(
		lingograph.Parallel(parallelTasks...),
		fitnessPrompt,
		lingograph.UserPrompt(p.userPromptCombine(numDays), false),
		actor.Pipeline(extra.Echoln(os.Stderr, "Final Plan\n\n"), !interactive, 3),
		actorOutputPlan.Pipeline(nil, false, 3),
	)

	chat := lingograph.NewChat()

	if interactive {
		pipeline = lingograph.Chain(pipeline, InteractivePipeline(actor))
	}

	err := pipeline.Execute(chat)

	if err != nil {
		util.Fatalf("error getting plan: %v\n", err)
	}
}

func (p Planner) SingleStep(interactive bool, numDays int) {
	args := p.templateMultiSportArgs(false)
	args["numDays"] = numDays
	args["order"] = "first"
	args["json_schema"] = fitness.JSONSchema()

	systemPrompt, err := p.templates.Execute("plan_single_step", args)
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	actor := actorOutputPlan(p.client, openai.GPT41, systemPrompt)

	pipeline := lingograph.Chain(
		lingograph.UserPrompt(userPromptFitness(p.fitness), false),
		actor.Pipeline(nil, false, 3),
	)

	if interactive {
		actorLoop := openai.NewActor(p.client, openai.GPT41, systemPrompt, nil)
		pipelineLoop := InteractivePipeline(actorLoop)
		pipeline = lingograph.Chain(pipeline, pipelineLoop)
	}

	chat := lingograph.NewChat()

	err = pipeline.Execute(chat)
	if err != nil {
		util.Fatalf("error getting plan: %v\n", err)
	}
}
