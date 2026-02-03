// engine/evaluator.go

package engine

import (
	"alertengine/common"
	"context"
	"time"
)

type QueryFunc func(ctx context.Context, expr string) (bool, float64, map[string]string, error)
type NotifyFunc func(rule EvalRule, state string)

type RuleEvaluator struct {
	rules      []EvalRule
	interval   time.Duration
	queryFunc  QueryFunc
	notifyFunc NotifyFunc
}

func (e *RuleEvaluator) UpdateRules(rules []EvalRule) {
	e.rules = rules
}

func (e *RuleEvaluator) Run(ctx context.Context) {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.evaluate(ctx)
		}
	}
}

func (e *RuleEvaluator) evaluate(ctx context.Context) {
	now := time.Now()

	for i := range e.rules {
		rule := &e.rules[i]

		hasValue, value, metricLabels, err := e.queryFunc(ctx, rule.Expr)
		if err != nil {
			continue
		}

		if hasValue && metricLabels != nil {
			mergedLabels := make(map[string]string)

			for _, label := range rule.Labels {
				mergedLabels[label.Name] = label.Value
			}

			for k, v := range metricLabels {
				mergedLabels[k] = v
			}
			
			rule.Labels = common.FromMap(mergedLabels)
		}

		e.updateRuleState(rule, hasValue, value, now)
	}
}

func (e *RuleEvaluator) updateRuleState(rule *EvalRule, hasValue bool, value float64, now time.Time) {
	rule.LastValue = value

	switch rule.State {
	case StateInactive:
		if hasValue {
			rule.State = StatePending
			rule.ActiveAt = now
		}

	case StatePending:
		if !hasValue {
			rule.State = StateInactive
			rule.ActiveAt = time.Time{}
		} else if now.Sub(rule.ActiveAt) >= rule.For {
			rule.State = StateFiring
			rule.FiredAt = now
			if e.notifyFunc != nil {
				e.notifyFunc(*rule, "firing")
			}
		}

	case StateFiring:
		if !hasValue {
			if e.notifyFunc != nil {
				e.notifyFunc(*rule, "resolved")
			}
			rule.State = StateInactive
			rule.ActiveAt = time.Time{}
			rule.FiredAt = time.Time{}
		} else {
			// 持续 firing
			if e.notifyFunc != nil {
				e.notifyFunc(*rule, "firing")
			}
		}
	}
}
