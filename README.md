# velora

**velora** is an AI-powered command-line workout tracker and coach, specializing in cycling and running. It combines workout logging with intelligent training recommendations.

## Quick Start

Log a workout using natural language:
```bash
$ velora addai 'Went on a ride this morning, covered 30km with 300m of climbing in 1h30min. It was a hilly route.'
read activity:

{
  "time": "2025-04-14T08:00:00+00:00",
  "sport": "cycling",
  "distance": 30000,
  "vertical_gain": 300,
  "duration": 5400,
  "was_recommended": false,
  "notes": "Hilly route"
}

does it look correct? (y/n) y
```

Review your recent activities:
```bash
$ velora recent
Date: Apr 13, 20:15
Sport: cycling
Time: 1h33m
Distance: 36.0km
Vertical Gain: 171m
Notes: Urban ride

Date: Apr 11, 18:00
Sport: running
Time: 52m
Distance: 8.8km
Vertical Gain: 42m
Notes: 

...
```

Get personalized training recommendations:
```bash
$ velora plan
Plan:

  - Date: 2025-04-14
    Sport: running
    Distance: 6000
    Notes: ...
  - Date: 2025-04-15
    Sport: cycling
    Distance: 50000
    Notes: ...
  - Date: 2025-04-16
    Sport: running
    Distance: 8000
    Notes: ...
```

Get detailed recommendations with multi-step analysis:
```bash
$ velora plan --multi-step
```
The multi-step mode:
- Analyzes your recent workout history
- Generates separate training plans for each sport (cycling and running)
- Combines the recommendations into a cohesive training schedule
- Provides detailed reasoning for each recommendation

Get insights about your training:
```bash
$ velora ask 'Evaluate my recent workouts. Are there signs of a plateau? What should I focus on?'
```

## Setup

`velora` uses OpenAI's API. Configure your API key:
```bash
export OPENAI_API_KEY="your-api-key-here"
```
You also need to copy the provided `prefs.json.sample` file to `~/.velora/prefs.json`, and then modify it to suit your preferences.

## AI Capabilities

### Current
- Natural language workout logging
- Single-prompt training plan generation
- Multi-step analysis for comprehensive recommendations
- Interactive Q&A about your training

### Future
1. ~~Fine-tuned LLMs for sport-specific recommendations~~ (now handled via OpenAI function calls for structured output; reasoning gains little from fine-tuning)
2. Integration of traditional machine learning approaches
3. Advanced training analytics and visualization
