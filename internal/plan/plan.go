package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/vasilisp/velora/internal/fitness"
	"github.com/vasilisp/velora/internal/langchain"
	"github.com/vasilisp/velora/internal/profile"
	"github.com/vasilisp/velora/internal/template"
	"github.com/vasilisp/velora/internal/util"
)

type Planner struct {
	client    langchain.Client
	fitness   *fitness.Fitness
	templates template.Parsed
}

func NewPlanner(client langchain.Client, fitness *fitness.Fitness) Planner {
	templates := []string{"header", "plan_*", "sched_*", "spec_*"}
	return Planner{client: client, fitness: fitness, templates: template.MakeParsed(templates)}
}

func (p Planner) systemPrompt() langchain.Message {
	systemPromptStr, err := p.templates.Execute("plan_system", nil)
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	return langchain.Message{
		Role:    langchain.MessageTypeSystem,
		Content: systemPromptStr,
	}
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

func (p Planner) userPromptOfSport(sport profile.Sport) (langchain.Message, allowedDisallowedDays) {
	days := nextThreeDays(p.fitness, p.fitness.Profile.AllowedDaysOfSport(sport))

	m := map[string]any{
		"allowed":    FormatDates(days.Allowed),
		"disallowed": FormatDates(days.Disallowed),
		"sport":      sport.String(),
	}

	if len(days.Allowed) == 0 {
		// no allowed days, no need to plan
		return langchain.Message{}, days
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

	return langchain.Message{
		Role:    langchain.MessageTypeHuman,
		Content: str,
	}, days
}

type sportData struct {
	UserPrompt langchain.Message
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

func (p Planner) userPromptCombine() langchain.Message {
	str, err := p.templates.Execute("plan_combine", p.templateMultiSportArgs(true))
	if err != nil {
		util.Fatalf("error getting combine user prompt: %v\n", err)
	}

	return langchain.Message{
		Role:    langchain.MessageTypeHuman,
		Content: str,
	}
}

func (p Planner) userPromptJSON() langchain.Message {
	str, err := p.templates.Execute("plan_json", nil)
	if err != nil {
		util.Fatalf("error getting JSON user prompt: %v\n", err)
	}

	return langchain.Message{
		Role:    langchain.MessageTypeHuman,
		Content: str,
	}
}

func userPromptFitness(fitness *fitness.Fitness) langchain.Message {
	bytes, err := json.MarshalIndent(fitness, "", "  ")
	if err != nil {
		util.Fatalf("error marshalling fitness data: %v\n", err)
	}

	return langchain.Message{
		Role:    langchain.MessageTypeHuman,
		Content: string(bytes),
	}
}

func (p Planner) singleSport(sport profile.Sport, userPrompt langchain.Message) {
	response, err := p.client.AskGPT([]langchain.Message{
		p.systemPrompt(),
		userPromptFitness(p.fitness),
		userPrompt,
	})
	if err != nil {
		util.Fatalf("error getting %s sport plan: %v\n", sport.String(), err)
	}

	fmt.Fprintf(os.Stderr, "## %s Plan\n\n%s\n\n", util.Capitalize(sport.String()), util.SanitizeOutput(response, false))

	responseJSON, err := p.client.AskGPT([]langchain.Message{
		userPrompt,
		{
			Role:    langchain.MessageTypeAI,
			Content: response,
		},
		p.userPromptJSON(),
	})
	if err != nil {
		util.Fatalf("error getting JSON plan: %v\n", err)
	}

	fmt.Println(util.SanitizeOutput(responseJSON, false))
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

	var wg sync.WaitGroup
	wg.Add(len(sportMap))

	askGPT := func(sport profile.Sport, sd *sportData) {
		defer wg.Done()
		var err error
		sd.Response, err = p.client.AskGPT([]langchain.Message{
			systemPrompt,
			userPromptFitness,
			sd.UserPrompt,
		})
		if err != nil {
			util.Fatalf("error getting %s plan: %v\n", sport.String(), err)
		}
	}

	for sport := range sportMap {
		go askGPT(sport, sportMap[sport])
	}

	wg.Wait()

	for sport, data := range sportMap {
		fmt.Fprintf(os.Stderr, "## %s Draft Plan\n\n%s\n\n", util.Capitalize(sport.String()), util.SanitizeOutput(data.Response, false))
	}

	userPromptCombine := p.userPromptCombine()

	messages := make([]langchain.Message, 2*len(sportMap)+3)
	messages[0] = systemPrompt
	messages[1] = userPromptFitness
	for i, data := range sportMap {
		messages[2*i+2] = data.UserPrompt
		messages[2*i+3] = langchain.Message{
			Role:    langchain.MessageTypeAI,
			Content: data.Response,
		}
	}
	messages[len(messages)-1] = userPromptCombine

	responseCombine, err := p.client.AskGPT(messages)
	if err != nil {
		util.Fatalf("error getting combine plan: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "## Final Combined Plan\n\n%s\n\n", util.SanitizeOutput(responseCombine, false))

	responseJSON, err := p.client.AskGPT([]langchain.Message{
		userPromptCombine,
		{
			Role:    langchain.MessageTypeAI,
			Content: responseCombine,
		},
		p.userPromptJSON(),
	})
	if err != nil {
		util.Fatalf("error getting JSON plan: %v\n", err)
	}

	fmt.Println(util.SanitizeOutput(responseJSON, false))
}

func (p Planner) SingleStep() {
	systemPromptStr, err := p.templates.Execute("plan_single_step", p.templateMultiSportArgs(false))
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}

	systemPrompt := langchain.Message{
		Role:    langchain.MessageTypeSystem,
		Content: systemPromptStr,
	}

	responseJSON, err := p.client.AskGPT([]langchain.Message{
		systemPrompt,
		userPromptFitness(p.fitness),
		p.userPromptJSON(),
	})
	if err != nil {
		util.Fatalf("error getting JSON plan: %v\n", err)
	}

	fmt.Println(util.SanitizeOutput(responseJSON, false))
}
