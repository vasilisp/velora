package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/vasilisp/velora/internal/fitness"
	"github.com/vasilisp/velora/internal/langchain"
	"github.com/vasilisp/velora/internal/util"
)

type Planner struct {
	client  langchain.Client
	fitness fitness.Fitness
}

func NewPlanner(client langchain.Client, fitness fitness.Fitness) Planner {
	return Planner{client: client, fitness: fitness}
}

func (p Planner) MultiStep() {
	systemPromptStr, err := util.ExecuteTemplate("plan_system", []string{"plan_system", "header", "spec_input"})
	if err != nil {
		util.Fatalf("error getting system prompt: %v\n", err)
	}
	systemPrompt := langchain.Message{
		Role:    langchain.MessageTypeSystem,
		Content: systemPromptStr,
	}

	userPromptCyclingStr, err := util.ExecuteTemplate("plan_cycling", []string{"plan_cycling"})
	if err != nil {
		util.Fatalf("error getting cycling user prompt: %v\n", err)
	}
	userPromptCycling := langchain.Message{
		Role:    langchain.MessageTypeHuman,
		Content: userPromptCyclingStr,
	}

	userPromptRunningStr, err := util.ExecuteTemplate("plan_running", []string{"plan_running"})
	if err != nil {
		util.Fatalf("error getting running user prompt: %v\n", err)
	}
	userPromptRunning := langchain.Message{
		Role:    langchain.MessageTypeHuman,
		Content: userPromptRunningStr,
	}

	userPromptCombineStr, err := util.ExecuteTemplate("plan_combine", []string{"plan_combine"})
	if err != nil {
		util.Fatalf("error getting combine user prompt: %v\n", err)
	}
	userPromptCombine := langchain.Message{
		Role:    langchain.MessageTypeHuman,
		Content: userPromptCombineStr,
	}

	userPromptJSONStr, err := util.ExecuteTemplate("plan_json", []string{"plan_json", "spec_output"})
	if err != nil {
		util.Fatalf("error getting JSON user prompt: %v\n", err)
	}
	userPromptJSON := langchain.Message{
		Role:    langchain.MessageTypeHuman,
		Content: userPromptJSONStr,
	}

	userPromptDate := langchain.Message{
		Role:    langchain.MessageTypeHuman,
		Content: fmt.Sprintf("**today is %s**\n", time.Now().Format("2006-01-02")),
	}

	fitnessBytes, err := json.MarshalIndent(p.fitness, "", "  ")
	if err != nil {
		util.Fatalf("error marshalling fitness data: %v\n", err)
	}

	userPromptFitness := langchain.Message{
		Role:    langchain.MessageTypeHuman,
		Content: string(fitnessBytes),
	}

	responseCycling, err := p.client.AskGPT([]langchain.Message{
		systemPrompt,
		userPromptDate,
		userPromptFitness,
		userPromptCycling,
	})
	if err != nil {
		util.Fatalf("error getting cycling plan: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "## Cycling Draft Plan\n\n%s\n\n", responseCycling)

	responseRunning, err := p.client.AskGPT([]langchain.Message{
		systemPrompt,
		userPromptDate,
		userPromptFitness,
		userPromptRunning,
	})
	if err != nil {
		util.Fatalf("error getting running plan: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "## Running Draft Plan\n\n%s\n\n", responseRunning)

	responseCombine, err := p.client.AskGPT([]langchain.Message{
		systemPrompt,
		userPromptDate,
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

	fmt.Fprintf(os.Stderr, "## Final Combined Plan\n\n%s\n\n", responseCombine)

	responseJSON, err := p.client.AskGPT([]langchain.Message{
		systemPrompt,
		userPromptCombine,
		{
			Role:    langchain.MessageTypeAI,
			Content: responseCombine,
		},
		userPromptJSON,
	})
	if err != nil {
		util.Fatalf("error getting JSON plan: %v\n", err)
	}

	fmt.Println(responseJSON)
}
