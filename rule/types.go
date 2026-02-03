package rule

import (
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type Prom struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}

type Rule struct {
	ID          int64             `json:"id"`
	PromID      int64             `json:"prom_id"`
	Expr        string            `json:"expr"`
	Op          string            `json:"op"`
	Value       string            `json:"value"`
	For         string            `json:"for"`
	Labels      map[string]string `json:"labels"`
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
}

type Rules []Rule

type PromRules struct {
	Prom  Prom  `json:"prom"`
	Rules Rules `json:"rules"`
}

type RulesResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data Rules  `json:"data"`
}

type PromsResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data []Prom `json:"data"`
}

type M map[string]interface{}

type S []interface{}

func (r Rules) Content() ([]byte, error) {
	rules := S{}
	for _, i := range r {
		rules = append(rules, M{
			"alert":  strconv.FormatInt(i.ID, 10),
			"expr":   strings.Join([]string{i.Expr, i.Op, i.Value}, " "),
			"for":    i.For,
			"labels": i.Labels,
			"annotations": M{
				"rule_id":     strconv.FormatInt(i.ID, 10),
				"prom_id":     strconv.FormatInt(i.PromID, 10),
				"summary":     i.Summary,
				"description": i.Description,
			},
		})
	}

	result := M{
		"groups": S{
			M{
				"name":  "ruleengine",
				"rules": rules,
			},
		},
	}

	return yaml.Marshal(result)
}

func (r Rules) PromRules() []PromRules {
	tmp := map[int64]Rules{}

	for _, rule := range r {
		if v, ok := tmp[rule.PromID]; ok {
			tmp[rule.PromID] = append(v, rule)
		} else {
			tmp[rule.PromID] = Rules{rule}
		}
	}

	data := []PromRules{}
	for id, rules := range tmp {
		data = append(data, PromRules{
			Prom:  Prom{ID: id},
			Rules: rules,
		})
	}

	return data
}

type RuleVersion struct {
	Version   int64     `json:"version"`
	PromID    int64     `json:"prom_id"`
	RuleCount int       `json:"rule_count"`
	CreatedAt time.Time `json:"created_at"`
	FilePath  string    `json:"file_path"`
	Hash      string    `json:"hash"`
}
