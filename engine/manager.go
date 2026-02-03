package engine

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"alertengine/config"
	"alertengine/rule"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"go.uber.org/zap"
)

// Manager 规则管理器
type Manager struct {
	prom      rule.Prom
	rules     rule.Rules
	config    *config.Config
	storage   *rule.Storage
	promAPI   v1.API
	evaluator *RuleEvaluator
	logger    *zap.Logger
	metrics   *Metrics
	ctx       context.Context
	cancel    context.CancelFunc
}

type RuleState int

const (
	StateInactive RuleState = iota
	StatePending
	StateFiring
)

// EvalRule 评估规则
type EvalRule struct {
	ID          string
	PromID      int64
	Expr        string
	For         time.Duration
	Labels      map[string]string
	Annotations map[string]string
	State       RuleState
	ActiveAt    time.Time
	FiredAt     time.Time
	LastValue   float64
}

// Alert 告警数据
type Alert struct {
	State       string            `json:"state"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Value       float64           `json:"value"`
	ActiveAt    string            `json:"active_at"`
	FiredAt     string            `json:"fired_at,omitempty"`
}

// NewManager 创建管理器
func NewManager(
	ctx context.Context,
	prom rule.Prom,
	cfg *config.Config,
	storage *rule.Storage,
	logger *zap.Logger,
	metrics *Metrics,
) (*Manager, error) {
	// 创建 Prometheus API 客户端
	var client api.Client
	var err error

	if cfg.AuthToken != "" {
		rt := &authRoundTripper{
			rt:    api.DefaultRoundTripper,
			token: cfg.AuthToken,
		}
		client, err = api.NewClient(api.Config{
			Address:      prom.URL,
			RoundTripper: rt,
		})
	} else {
		client, err = api.NewClient(api.Config{
			Address: prom.URL,
		})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	promAPI := v1.NewAPI(client)
	mgrCtx, cancel := context.WithCancel(ctx)

	m := &Manager{
		prom:    prom,
		config:  cfg,
		storage: storage,
		promAPI: promAPI,
		logger:  logger,
		metrics: metrics,
		ctx:     mgrCtx,
		cancel:  cancel,
	}

	// 创建评估器
	m.evaluator = &RuleEvaluator{
		rules:      []EvalRule{},
		interval:   time.Duration(cfg.EvaluationInterval),
		queryFunc:  m.queryPrometheus,
		notifyFunc: m.sendNotification,
	}

	return m, nil
}

// Update 更新规则
func (m *Manager) Update(rules rule.Rules) error {
	m.rules = rules

	// 生成规则内容并保存
	content, err := rules.Content()
	if err != nil {
		m.logger.Error("failed to generate rule content",
			zap.Int64("prom_id", m.prom.ID),
			zap.Error(err),
		)
		return err
	}

	_, err = m.storage.SaveRule(m.prom.ID, content)
	if err != nil {
		m.logger.Error("failed to save rule file",
			zap.Int64("prom_id", m.prom.ID),
			zap.Error(err),
		)
		return err
	}

	// 转换为评估规则
	evalRules := make([]EvalRule, len(rules))
	for i, r := range rules {
		forDuration, _ := time.ParseDuration(r.For)
		evalRules[i] = EvalRule{
			ID:     strconv.FormatInt(r.ID, 10),
			PromID: r.PromID,
			Expr:   strings.TrimSpace(r.Expr + " " + r.Op + " " + r.Value),
			For:    forDuration,
			Labels: r.Labels,
			Annotations: map[string]string{
				"rule_id":     strconv.FormatInt(r.ID, 10),
				"prom_id":     strconv.FormatInt(r.PromID, 10),
				"summary":     r.Summary,
				"description": r.Description,
			},
			State: StateInactive,
		}
	}

	m.evaluator.UpdateRules(evalRules)
	m.metrics.RulesLoaded.WithLabelValues(fmt.Sprintf("%d", m.prom.ID)).Set(float64(len(rules)))

	m.logger.Info("rules updated successfully",
		zap.Int64("prom_id", m.prom.ID),
		zap.Int("rule_count", len(rules)),
	)

	return nil
}

// Run 运行管理器
func (m *Manager) Run() {
	m.logger.Info("starting rule manager", zap.Int64("prom_id", m.prom.ID))
	go m.evaluator.Run(m.ctx)
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.logger.Info("stopping rule manager", zap.Int64("prom_id", m.prom.ID))
	m.cancel()
}

// queryPrometheus 查询 Prometheus
func (m *Manager) queryPrometheus(ctx context.Context, expr string) (bool, float64, error) {
	value, _, err := m.promAPI.Query(ctx, expr, time.Now())
	if err != nil {
		m.logger.Debug("query failed",
			zap.String("expr", expr),
			zap.Error(err),
		)
		return false, 0, err
	}

	switch v := value.(type) {
	case model.Vector:
		if len(v) > 0 {
			return true, float64(v[0].Value), nil
		}
		return false, 0, nil
	case *model.Scalar:
		return true, float64(v.Value), nil
	default:
		return false, 0, nil
	}
}

// sendNotification 发送通知
func (m *Manager) sendNotification(rule EvalRule, state string) {
	alert := Alert{
		State:       state,
		Labels:      rule.Labels,
		Annotations: rule.Annotations,
		Value:       math.Round(rule.LastValue*100) / 100,
		ActiveAt:    rule.ActiveAt.Format(time.RFC3339),
	}

	if !rule.FiredAt.IsZero() {
		alert.FiredAt = rule.FiredAt.Format(time.RFC3339)
	}

	//data, err := json.Marshal([]Alert{alert})
	//if err != nil {
	//	m.logger.Error("failed to marshal alert", zap.Error(err))
	//	m.metrics.NotifyErrors.Inc()
	//	return
	//}

	//url := fmt.Sprintf("%s%s", m.config.Gateway.URL, m.config.Gateway.NotifyPath)

	for i := 1; i <= m.config.NotifyRetries; i++ {
		fmt.Println("alert-data", alert)
		//client := &http.Client{Timeout: 5 * time.Second}
		//req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
		//req.Header.Set("Token", m.config.AuthToken)
		//req.Header.Set("Content-Type", "application/json")
		//
		//resp, err := client.Do(req)
		//if err != nil {
		//	m.logger.Error("notify failed",
		//		zap.String("url", url),
		//		zap.Int("retry", i),
		//		zap.Error(err),
		//	)
		//	m.metrics.NotifyErrors.Inc()
		//	continue
		//}
		//
		//if resp.StatusCode == 200 {
		//	io.Copy(io.Discard, resp.Body)
		//	resp.Body.Close()
		//	m.logger.Debug("notify succeeded", zap.String("url", url))
		//	m.metrics.NotificationsSent.Add(1)
		//	return
		//}
		//
		//io.Copy(io.Discard, resp.Body)
		//resp.Body.Close()

		//m.logger.Error("notify failed",
		//	zap.String("url", url),
		//	//zap.Int("status", resp.StatusCode),
		//	zap.Int("retry", i),
		//)
		//m.metrics.NotifyErrors.Inc()
		m.metrics.NotificationsSent.Add(1)
	}
}

// authRoundTripper HTTP认证
type authRoundTripper struct {
	rt    http.RoundTripper
	token string
}

func (rt *authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Basic "+rt.token)
	return rt.rt.RoundTrip(req)
}
