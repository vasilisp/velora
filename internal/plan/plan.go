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
	Date     string       `json:"date" jsonschema:"description=The date of the planned workout in YYYY-MM-DD format"`
	Sport    string       `json:"sport" jsonschema:"description=The type of sport (running, cycling, swimming)"`
	Distance int          `json:"distance" jsonschema:"description=The planned distance in meters"`
	Notes    string       `json:"notes" jsonschema:"description=Additional notes and instructions for the workout, in one line"`
	Segments []db.Segment `json:"segments" jsonschema:"description=The segments of the workout"`
}

type Plan struct {
	Days        []PlanDay `json:"days"`
	Explanation string    `json:"explanation" jsonschema:"description=A short explanation of the choices made, in one paragraph maximum"`
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
	systemPromptStr, err := p.templates.Execute("plan_system", nil)
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

func nextThreeDays(fitness *fitness.Fitness, allowedDays profile.AllowedDays) allowedDisallowedDays {
	today := time.Now()
	startDate := today

	days := allowedDisallowedDays{
		Allowed:    []time.Time{},
		Disallowed: []time.Time{},
	}

	// Check if there's an activity today
	for _, activity := range fitness.ActivitiesThisWeek {
		if activity.Time.Format("2006-01-02") == today.Format("2006-01-02") {
			// Activity found today, start planning from tomorrow
			startDate = today.AddDate(0, 0, 1)
			break
		}
	}

	for i := range 3 {
		date := startDate.AddDate(0, 0, i)
		if _, ok := allowedDays[date.Weekday()]; ok {
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

func (p Planner) userPromptOfSport(sport profile.Sport) (string, allowedDisallowedDays) {
	days := nextThreeDays(p.fitness, p.fitness.Profile.AllowedDaysOfSport(sport))

	m := map[string]any{
		"allowed":    FormatDates(days.Allowed),
		"disallowed": FormatDates(days.Disallowed),
		"sport":      sport.String(),
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
		allowedDisallowedDays := nextThreeDays(p.fitness, p.fitness.Profile.AllowedDaysOfSport(sport))

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

func (p Planner) userPromptCombine() string {
	str, err := p.templates.Execute("plan_combine", p.templateMultiSportArgs(true))
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

func (p Planner) MultiStep() {
	sportMap := make(map[profile.Sport]*sportData)

	for _, sport := range p.fitness.Profile.AllSports() {
		userPrompt, days := p.userPromptOfSport(sport)
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

	actorSingleSport := openai.NewActor(p.client, openai.GPT41Mini, systemPrompt, &temperature)

	fitnessPrompt := lingograph.UserPrompt(userPromptFitness, false)

	parallelTasks := make([]lingograph.Pipeline, 0, len(sportMap))
	for sport, data := range sportMap {
		parallelTasks = append(parallelTasks, lingograph.Chain(
			fitnessPrompt,
			lingograph.UserPrompt(data.UserPrompt, false),
			actorSingleSport.Pipeline(
				extra.Echoln(os.Stderr, fmt.Sprintf("%s Draft Plan\n\n", util.Capitalize(sport.String()))),
				true,
				3,
			),
		))
	}

	actorCombine := openai.NewActor(p.client, openai.GPT41, systemPrompt, &temperature)
	actorOutputPlan := actorOutputPlan(p.client, openai.GPT41Nano, systemPromptSummarize)

	pipeline := lingograph.Chain(
		lingograph.Parallel(parallelTasks...),
		fitnessPrompt,
		lingograph.UserPrompt(p.userPromptCombine(), false),
		actorCombine.Pipeline(extra.Echoln(os.Stderr, "Final Plan\n\n"), true, 3),
		actorOutputPlan.Pipeline(nil, false, 3),
	)

	chat := lingograph.NewChat()

	err := pipeline.Execute(chat)

	if err != nil {
		util.Fatalf("error getting plan: %v\n", err)
	}
}

func (p Planner) SingleStep() {
	systemPrompt, err := p.templates.Execute("plan_single_step", p.templateMultiSportArgs(false))
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	actor := actorOutputPlan(p.client, openai.GPT41, systemPrompt)

	pipeline := lingograph.Chain(
		lingograph.UserPrompt(userPromptFitness(p.fitness), false),
		actor.Pipeline(nil, false, 3),
	)

	chat := lingograph.NewChat()

	err = pipeline.Execute(chat)
	if err != nil {
		util.Fatalf("error getting plan: %v\n", err)
	}
}
