package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"alertengine/config"
	"alertengine/rule"

	"go.uber.org/zap"
)

// Reloader 规则重载器
type Reloader struct {
	config   *config.Config
	storage  *rule.Storage
	managers map[int64]*Manager
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	running  bool
	logger   *zap.Logger
	metrics  *Metrics
}

// NewReloader 创建重载器
func NewReloader(
	cfg *config.Config,
	storage *rule.Storage,
	logger *zap.Logger,
	metrics *Metrics,
) *Reloader {
	ctx, cancel := context.WithCancel(context.Background())

	return &Reloader{
		config:   cfg,
		storage:  storage,
		managers: make(map[int64]*Manager),
		ctx:      ctx,
		cancel:   cancel,
		running:  false,
		logger:   logger,
		metrics:  metrics,
	}
}

// Run 启动重载器
func (r *Reloader) Run() {
	r.mu.Lock()
	r.running = true
	r.mu.Unlock()

	r.logger.Info("reloader started")

	// 启动所有已存在的管理器
	r.mu.RLock()
	for _, manager := range r.managers {
		manager.Run()
	}
	r.mu.RUnlock()
}

// Stop 停止重载器
func (r *Reloader) Stop() {
	r.mu.Lock()
	r.running = false
	r.mu.Unlock()

	r.cancel()

	// 停止所有管理器
	r.mu.RLock()
	for _, manager := range r.managers {
		manager.Stop()
	}
	r.mu.RUnlock()

	r.logger.Info("reloader stopped")
}

// Loop 主循环
func (r *Reloader) Loop() {
	ticker := time.NewTicker(time.Duration(r.config.ReloadInterval))
	defer ticker.Stop()

	// 立即执行一次
	if err := r.Update(); err != nil {
		r.logger.Error("initial update failed", zap.Error(err))
	}

	for r.running {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			if err := r.Update(); err != nil {
				r.logger.Error("update failed", zap.Error(err))
				r.metrics.ReloadErrors.Inc()
			} else {
				r.metrics.ReloadSuccess.Inc()
			}
		}
	}
}

// Update 更新规则
func (r *Reloader) Update() error {
	r.logger.Info("starting rule update")

	// 获取规则和数据源
	promRules, err := r.fetchPromRules()
	if err != nil {
		return fmt.Errorf("failed to fetch rules: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 停止无效的管理器
	for id, manager := range r.managers {
		shouldDelete := true
		for _, pr := range promRules {
			if manager.prom.ID == pr.Prom.ID &&
				manager.prom.URL == pr.Prom.URL &&
				pr.Prom.URL != "" {
				shouldDelete = false
				break
			}
		}

		if shouldDelete {
			r.logger.Info("removing obsolete manager",
				zap.Int64("prom_id", manager.prom.ID),
				zap.String("prom_url", manager.prom.URL),
			)
			manager.Stop()
			delete(r.managers, id)
		}
	}

	// 更新或创建管理器
	for _, pr := range promRules {
		if pr.Prom.URL == "" {
			r.logger.Warn("skipping prom with empty URL", zap.Int64("prom_id", pr.Prom.ID))
			continue
		}

		manager, exists := r.managers[pr.Prom.ID]

		// 创建新管理器
		if !exists {
			newManager, err := NewManager(
				r.ctx,
				pr.Prom,
				r.config,
				r.storage,
				r.logger,
				r.metrics,
			)
			if err != nil {
				r.logger.Error("failed to create manager",
					zap.Int64("prom_id", pr.Prom.ID),
					zap.Error(err),
				)
				continue
			}

			newManager.Run()
			r.managers[pr.Prom.ID] = newManager
			manager = newManager
		}

		// 更新规则
		if err := manager.Update(pr.Rules); err != nil {
			r.logger.Error("failed to update rules",
				zap.Int64("prom_id", manager.prom.ID),
				zap.Error(err),
			)
		} else {
			r.logger.Info("rules updated",
				zap.Int64("prom_id", manager.prom.ID),
				zap.Int("count", len(pr.Rules)),
			)
		}
	}

	r.logger.Info("rule update completed",
		zap.Int("manager_count", len(r.managers)),
	)

	return nil
}

// fetchPromRules 获取规则和数据源
func (r *Reloader) fetchPromRules() ([]rule.PromRules, error) {
	client := &http.Client{
		Timeout: r.config.Gateway.Timeout,
	}

	// 获取规则列表
	rules, err := r.fetchRules(client)
	if err != nil {
		return nil, err
	}

	// 获取数据源列表
	proms, err := r.fetchProms(client)
	if err != nil {
		return nil, err
	}

	// 组合数据
	promRules := rules.PromRules()
	for i := range promRules {
		for _, prom := range proms {
			if promRules[i].Prom.ID == prom.ID {
				promRules[i].Prom.URL = prom.URL
				break
			}
		}
	}

	return promRules, nil
}

// fetchRules 获取规则列表
func (r *Reloader) fetchRules(client *http.Client) (rule.Rules, error) {
	url := fmt.Sprintf("%s%s", r.config.Gateway.URL, r.config.Gateway.RulePath)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Token", r.config.AuthToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var rulesResp rule.RulesResp
	if err := json.NewDecoder(resp.Body).Decode(&rulesResp); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	if rulesResp.Code != 0 {
		return nil, fmt.Errorf("api error: %s", rulesResp.Msg)
	}

	r.logger.Info("rules fetched", zap.Int("count", len(rulesResp.Data)))
	return rulesResp.Data, nil
}

// fetchProms 获取数据源列表
func (r *Reloader) fetchProms(client *http.Client) ([]rule.Prom, error) {
	url := fmt.Sprintf("%s%s", r.config.Gateway.URL, r.config.Gateway.PromPath)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Token", r.config.AuthToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var promsResp rule.PromsResp
	if err := json.NewDecoder(resp.Body).Decode(&promsResp); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	if promsResp.Code != 0 {
		return nil, fmt.Errorf("api error: %s", promsResp.Msg)
	}

	r.logger.Info("proms fetched", zap.Int("count", len(promsResp.Data)))
	return promsResp.Data, nil
}

// GetManagerCount 获取管理器数量
func (r *Reloader) GetManagerCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.managers)
}
