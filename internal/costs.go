package internal

import (
	"fmt"
	"strings"
	"time"
)

type ModelPricing struct {
	InputPer1K  float64
	OutputPer1K float64
}

var PricingTable = map[string]ModelPricing{
	"gpt-4o":               {0.0025, 0.01},
	"gpt-4o-mini":          {0.00015, 0.0006},
	"gpt-4-turbo":          {0.01, 0.03},
	"o1":                   {0.015, 0.06},
	"o1-mini":              {0.003, 0.012},
	"claude-sonnet-4-20250514": {0.003, 0.015},
	"claude-3-5-haiku-20241022": {0.0008, 0.004},
	"claude-3-opus-20240229":   {0.015, 0.075},
}

type UsageRecord struct {
	Timestamp    time.Time
	Model        string
	InputTokens  int
	OutputTokens int
	Cost         float64
}

type CostTracker struct {
	Records     []UsageRecord
	TotalCost   float64
	SessionCost float64
	StartTime   time.Time
}

func NewCostTracker() *CostTracker {
	return &CostTracker{
		Records:   make([]UsageRecord, 0),
		StartTime: time.Now(),
	}
}

func (ct *CostTracker) Track(model string, inputTokens, outputTokens int) {
	pricing, ok := PricingTable[model]
	if !ok {
		pricing = ModelPricing{0.001, 0.002}
	}

	cost := (float64(inputTokens) / 1000 * pricing.InputPer1K) +
		(float64(outputTokens) / 1000 * pricing.OutputPer1K)

	record := UsageRecord{
		Timestamp:    time.Now(),
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
	}

	ct.Records = append(ct.Records, record)
	ct.TotalCost += cost
	ct.SessionCost += cost
}

func (ct *CostTracker) EstimateTokens(text string) int {
	return len(text) / 4
}

func (ct *CostTracker) GetSessionSummary() string {
	if len(ct.Records) == 0 {
		return "No usage recorded."
	}

	var out strings.Builder
	out.WriteString("\033[1;35mSession Usage:\033[0m\n\n")

	modelUsage := make(map[string]struct {
		Tokens int
		Cost   float64
		Count  int
	})

	for _, r := range ct.Records {
		usage := modelUsage[r.Model]
		usage.Tokens += r.InputTokens + r.OutputTokens
		usage.Cost += r.Cost
		usage.Count++
		modelUsage[r.Model] = usage
	}

	for model, usage := range modelUsage {
		out.WriteString(fmt.Sprintf("  \033[1;36m%s\033[0m: %d requests, ~%d tokens, $%.4f\n",
			model, usage.Count, usage.Tokens, usage.Cost))
	}

	out.WriteString(fmt.Sprintf("\n  \033[1;33mTotal: $%.4f\033[0m\n", ct.SessionCost))

	duration := time.Since(ct.StartTime)
	out.WriteString(fmt.Sprintf("  Duration: %s\n", duration.Round(time.Second)))

	return out.String()
}

func (ct *CostTracker) GetDailySummary() string {
	today := time.Now().Format("2006-01-02")
	var dailyCost float64
	var dailyRequests int

	for _, r := range ct.Records {
		if r.Timestamp.Format("2006-01-02") == today {
			dailyCost += r.Cost
			dailyRequests++
		}
	}

	return fmt.Sprintf("\033[1;35mToday (%s):\033[0m %d requests, $%.4f", today, dailyRequests, dailyCost)
}

func (ct *CostTracker) RenderBar() string {
	if ct.SessionCost == 0 {
		return ""
	}

	maxCost := 1.0
	barWidth := 30

	filled := int((ct.SessionCost / maxCost) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	return fmt.Sprintf("\033[1;33m[$%0.4f]\033[0m %s", ct.SessionCost, bar)
}
