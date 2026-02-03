package engine

import (
	"context"
	"time"
)

// RuleEvaluator 规则评估器
type RuleEvaluator struct {
	rules      []EvalRule
	interval   time.Duration
	queryFunc  func(ctx context.Context, expr string) (bool, float64, error)
	notifyFunc func(rule EvalRule, state string)
}

// UpdateRules 更新规则
func (e *RuleEvaluator) UpdateRules(rules []EvalRule) {
	e.rules = rules
}

// Run 运行评估循环
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

// evaluate 执行评估
func (e *RuleEvaluator) evaluate(ctx context.Context) {
	now := time.Now()

	for i := range e.rules {
		rule := &e.rules[i]

		// 查询 Prometheus
		hasValue, value, err := e.queryFunc(ctx, rule.Expr)
		if err != nil {
			continue
		}

		rule.LastValue = value

		// 状态机转换
		e.updateRuleState(rule, hasValue, now)
	}
}

// updateRuleState 更新规则状态
func (e *RuleEvaluator) updateRuleState(rule *EvalRule, hasValue bool, now time.Time) {
	switch rule.State {
	case StateInactive:
		if hasValue {
			rule.State = StatePending
			rule.ActiveAt = now
		}

	case StatePending:
		if !hasValue {
			// 条件不再满足，回到 inactive
			rule.State = StateInactive
			rule.ActiveAt = time.Time{}
		} else if now.Sub(rule.ActiveAt) >= rule.For {
			// 持续时间达到，进入 firing
			rule.State = StateFiring
			rule.FiredAt = now
			if e.notifyFunc != nil {
				e.notifyFunc(*rule, "firing")
			}
		}

	case StateFiring:
		if !hasValue {
			// 条件不再满足，发送 resolved 通知
			if e.notifyFunc != nil {
				e.notifyFunc(*rule, "resolved")
			}
			rule.State = StateInactive
			rule.ActiveAt = time.Time{}
			rule.FiredAt = time.Time{}
		}
		// firing 状态下持续满足条件，保持状态
	}
}
