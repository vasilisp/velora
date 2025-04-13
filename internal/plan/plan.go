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
	allowed    []time.Time
	disallowed []time.Time
}

func nextThreeDays(fitness *fitness.Fitness, allowedDays profile.AllowedDays) allowedDisallowedDays {
	today := time.Now()
	startDate := today

	days := allowedDisallowedDays{
		allowed:    []time.Time{},
		disallowed: []time.Time{},
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
			days.allowed = append(days.allowed, date)
		} else {
			days.disallowed = append(days.disallowed, date)
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

func (p Planner) userPromptOfSport(sport profile.Sport) (langchain.Message, map[string][]string) {
	days := nextThreeDays(p.fitness, p.fitness.Profile.AllowedDaysOfSport(sport))

	m := map[string][]string{
		"allowed":    FormatDates(days.allowed),
		"disallowed": FormatDates(days.disallowed),
	}

	if len(days.allowed) == 0 {
		// no allowed days, no need to plan
		return langchain.Message{}, m
	}

	var str string
	var err error
	switch sport {
	case profile.Cycling:
		str, err = p.templates.Execute("plan_cycling", m)
		if err != nil {
			util.Fatalf("error getting cycling template: %v\n", err)
		}
	case profile.Running:
		str, err = p.templates.Execute("plan_running", m)
		if err != nil {
			util.Fatalf("error getting running template: %v\n", err)
		}
	default:
		util.Fatalf("invalid sport: %d", sport)
	}

	return langchain.Message{
		Role:    langchain.MessageTypeHuman,
		Content: str,
	}, m
}

func (p Planner) userPromptCombine(daysCycling map[string][]string, daysRunning map[string][]string) langchain.Message {
	util.Assert(len(daysCycling["allowed"]) > 0 || len(daysRunning["allowed"]) > 0, "no allowed days")

	m := map[string]any{
		"allowedCycling":    daysCycling["allowed"],
		"allowedRunning":    daysRunning["allowed"],
		"disallowedCycling": daysCycling["disallowed"],
		"disallowedRunning": daysRunning["disallowed"],
	}

	str, err := p.templates.Execute("plan_combine", m)
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

func (p Planner) singleSport(userPrompt langchain.Message, sport profile.Sport) {
	response, err := p.client.AskGPT([]langchain.Message{
		p.systemPrompt(),
		userPromptFitness(p.fitness),
		userPrompt,
	})
	if err != nil {
		util.Fatalf("error getting %s sport plan: %v\n", sport.String(), err)
	}

	fmt.Fprintf(os.Stderr, "## %s Plan\n\n%s\n\n", sport.String(), util.SanitizeOutput(response, false))

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
	userPromptCycling, daysCycling := p.userPromptOfSport(profile.Cycling)
	userPromptRunning, daysRunning := p.userPromptOfSport(profile.Running)

	if len(daysCycling["allowed"]) == 0 && len(daysRunning["allowed"]) == 0 {
		fmt.Println("[]")
		return
	}

	if len(daysRunning["allowed"]) == 0 {
		p.singleSport(userPromptCycling, profile.Cycling)
		return
	}

	if len(daysCycling["allowed"]) == 0 {
		p.singleSport(userPromptRunning, profile.Running)
		return
	}

	systemPrompt := p.systemPrompt()
	userPromptFitness := userPromptFitness(p.fitness)
	userPromptCombine := p.userPromptCombine(daysCycling, daysRunning)

	var wg sync.WaitGroup
	var responseCycling string
	var responseRunning string

	wg.Add(2)

	askGPT := func(label string, prompt langchain.Message, response *string) {
		defer wg.Done()
		var err error
		*response, err = p.client.AskGPT([]langchain.Message{
			systemPrompt,
			userPromptFitness,
			prompt,
		})
		if err != nil {
			util.Fatalf("error getting %s plan: %v\n", label, err)
		}
	}

	go askGPT("cycling", userPromptCycling, &responseCycling)
	go askGPT("running", userPromptRunning, &responseRunning)
	wg.Wait()

	fmt.Fprintf(os.Stderr, "## Cycling Draft Plan\n\n%s\n\n", util.SanitizeOutput(responseCycling, false))
	fmt.Fprintf(os.Stderr, "## Running Draft Plan\n\n%s\n\n", util.SanitizeOutput(responseRunning, false))

	responseCombine, err := p.client.AskGPT([]langchain.Message{
		systemPrompt,
		userPromptFitness,
		userPromptCycling,
		{
			Role:    langchain.MessageTypeAI,
			Content: responseCycling,
		},
		userPromptRunning,
		{
			Role:    langchain.MessageTypeAI,
			Content: responseRunning,
		},
		userPromptCombine,
	})
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
	daysCycling := nextThreeDays(p.fitness, p.fitness.Profile.AllowedDaysOfSport(profile.Cycling))
	daysRunning := nextThreeDays(p.fitness, p.fitness.Profile.AllowedDaysOfSport(profile.Running))
	m := map[string]any{
		"allowedCycling":    FormatDates(daysCycling.allowed),
		"allowedRunning":    FormatDates(daysRunning.allowed),
		"disallowedCycling": FormatDates(daysCycling.disallowed),
		"disallowedRunning": FormatDates(daysRunning.disallowed),
	}

	systemPromptStr, err := p.templates.Execute("plan_single_step", m)
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
