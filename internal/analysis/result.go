package analysis

import "fmt"

type Stat struct {
	Label string `json:"label" yaml:"label"`
	Value string `json:"value" yaml:"value"`
}

type Column struct {
	Key   string `json:"key" yaml:"key"`
	Label string `json:"label" yaml:"label"`
}

type Dataset struct {
	Title   string           `json:"title" yaml:"title"`
	Summary []Stat           `json:"summary,omitempty" yaml:"summary,omitempty"`
	Columns []Column         `json:"columns,omitempty" yaml:"columns,omitempty"`
	Records []map[string]any `json:"records,omitempty" yaml:"records,omitempty"`
	Hints   []string         `json:"hints,omitempty" yaml:"hints,omitempty"`
}

func (d *Dataset) CloneWithLimit(limit int) (Dataset, int) {
	if d == nil {
		return Dataset{}, 0
	}

	cloned := Dataset{
		Title:   d.Title,
		Summary: append([]Stat(nil), d.Summary...),
		Columns: append([]Column(nil), d.Columns...),
		Hints:   append([]string(nil), d.Hints...),
	}

	total := len(d.Records)
	if limit <= 0 || limit >= total {
		cloned.Records = cloneRecords(d.Records)
		return cloned, total
	}

	cloned.Records = cloneRecords(d.Records[:limit])
	return cloned, total
}

func cloneRecords(in []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, row := range in {
		cloned := make(map[string]any, len(row))
		for k, v := range row {
			cloned[k] = v
		}
		out = append(out, cloned)
	}
	return out
}

func StringValue(v any) string {
	switch value := v.(type) {
	case nil:
		return "-"
	case string:
		if value == "" {
			return "-"
		}
		return value
	case fmt.Stringer:
		return value.String()
	default:
		return fmt.Sprintf("%v", value)
	}
}
