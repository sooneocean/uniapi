package usage

type ModelPricing struct {
	InputPerM  float64
	OutputPerM float64
}

var DefaultPricing = map[string]ModelPricing{
	"claude-sonnet-4-20250514": {InputPerM: 3.0, OutputPerM: 15.0},
	"claude-haiku-4-20250414":  {InputPerM: 0.8, OutputPerM: 4.0},
	"gpt-4o":                   {InputPerM: 2.5, OutputPerM: 10.0},
	"gpt-4o-mini":              {InputPerM: 0.15, OutputPerM: 0.6},
	"gemini-2.5-pro":           {InputPerM: 1.25, OutputPerM: 10.0},
}

func CalculateCost(model string, tokensIn, tokensOut int) float64 {
	p, ok := DefaultPricing[model]
	if !ok {
		return 0
	}
	return (float64(tokensIn)/1_000_000)*p.InputPerM + (float64(tokensOut)/1_000_000)*p.OutputPerM
}
