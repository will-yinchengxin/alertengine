package rule

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"go.uber.org/zap"
)

// Storage 规则存储管理器
type Storage struct {
	baseDir       string
	retentionDays int
	enableHistory bool
	logger        *zap.Logger
}

func NewStorage(baseDir string, retentionDays int, enableHistory bool, logger *zap.Logger) (*Storage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &Storage{
		baseDir:       baseDir,
		retentionDays: retentionDays,
		enableHistory: enableHistory,
		logger:        logger,
	}, nil
}

func (s *Storage) SaveRule(promID int64, content []byte) (string, error) {
	hash := s.calculateHash(content)

	var filepath string
	if s.enableHistory {
		timestamp := time.Now().Format("20060102_150405")
		filepath = s.getHistoryPath(promID, timestamp)
	} else {
		filepath = s.getCurrentPath(promID)
	}

	// 确保目录存在
	dir := filepath[:len(filepath)-len(filepath[len(filepath)-1:])]
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(filepath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	s.logger.Info("rule saved",
		zap.Int64("prom_id", promID),
		zap.String("path", filepath),
		zap.String("hash", hash),
	)

	return filepath, nil
}

func (s *Storage) GetCurrentRule(promID int64) string {
	return s.getCurrentPath(promID)
}

func (s *Storage) ListVersions(promID int64, limit int) ([]RuleVersion, error) {
	if !s.enableHistory {
		return nil, fmt.Errorf("history is disabled")
	}

	historyDir := s.getPromHistoryDir(promID)
	files, err := ioutil.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []RuleVersion{}, nil
		}
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().After(files[j].ModTime())
	})

	versions := []RuleVersion{}
	for i, file := range files {
		if i >= limit && limit > 0 {
			break
		}

		if file.IsDir() {
			continue
		}

		content, err := os.ReadFile(filepath.Join(historyDir, file.Name()))
		if err != nil {
			continue
		}

		versions = append(versions, RuleVersion{
			Version:   int64(i + 1),
			PromID:    promID,
			CreatedAt: file.ModTime(),
			FilePath:  filepath.Join(historyDir, file.Name()),
			Hash:      s.calculateHash(content),
		})
	}

	return versions, nil
}

func (s *Storage) CleanupOldVersions() error {
	if !s.enableHistory {
		return nil
	}

	cutoffTime := time.Now().AddDate(0, 0, -s.retentionDays)
	promDirs, err := os.ReadDir(s.baseDir)
	if err != nil {
		return fmt.Errorf("failed to read base directory: %w", err)
	}

	deletedCount := 0
	for _, promDir := range promDirs {
		if !promDir.IsDir() {
			continue
		}

		historyDir := filepath.Join(s.baseDir, promDir.Name(), "history")
		files, err := ioutil.ReadDir(historyDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			s.logger.Error("failed to read history directory",
				zap.String("dir", historyDir),
				zap.Error(err),
			)
			continue
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			if file.ModTime().Before(cutoffTime) {
				filePath := filepath.Join(historyDir, file.Name())
				if err := os.Remove(filePath); err != nil {
					s.logger.Error("failed to remove old version",
						zap.String("path", filePath),
						zap.Error(err),
					)
					continue
				}
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		s.logger.Info("cleaned up old rule versions",
			zap.Int("deleted_count", deletedCount),
			zap.Int("retention_days", s.retentionDays),
		)
	}

	return nil
}

func (s *Storage) calculateHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func (s *Storage) getCurrentPath(promID int64) string {
	return filepath.Join(s.baseDir, fmt.Sprintf("prom_%d", promID), "current.yml")
}

func (s *Storage) getHistoryPath(promID int64, timestamp string) string {
	return filepath.Join(
		s.baseDir,
		fmt.Sprintf("prom_%d", promID),
		"history",
		fmt.Sprintf("rule_%s.yml", timestamp),
	)
}

func (s *Storage) getPromHistoryDir(promID int64) string {
	return filepath.Join(s.baseDir, fmt.Sprintf("prom_%d", promID), "history")
}
