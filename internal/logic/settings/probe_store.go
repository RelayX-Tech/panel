package settings

import (
	"context"
	"encoding/json"
	"time"

	"github.com/relayx-tech/panel/internal/model"
)

const probeSettingKey = "probe"

// probeSettings 是 panel_settings 中 key='probe' 的存储形态。
type probeSettings struct {
	FrequencySeconds  int    `json:"frequencySeconds"`
	TimeoutMs         int    `json:"timeoutMs"`
	FailureThreshold  int    `json:"failureThreshold"`
	RecoveryThreshold int    `json:"recoveryThreshold"`
	UpdatedAt         string `json:"updatedAt"`
}

func defaultProbeSettings() probeSettings {
	return probeSettings{
		FrequencySeconds: 30, TimeoutMs: 5000, FailureThreshold: 3, RecoveryThreshold: 3,
	}
}

func loadProbeSettings(ctx context.Context, store model.SettingModel) (probeSettings, error) {
	settings := defaultProbeSettings()
	raw, found, err := store.GetJSON(ctx, probeSettingKey)
	if err != nil {
		return settings, err
	}
	if !found || len(raw) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		// 损坏的配置回退默认值，不阻塞页面。
		return defaultProbeSettings(), nil
	}
	return settings, nil
}

func saveProbeSettings(ctx context.Context, store model.SettingModel, settings probeSettings) error {
	settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return store.SetJSON(ctx, probeSettingKey, raw)
}
