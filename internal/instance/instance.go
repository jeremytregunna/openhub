package instance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type Instance struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

func LoadOrCreate(storagePath string) (*Instance, error) {
	instancePath := filepath.Join(storagePath, "instance.json")

	if data, err := os.ReadFile(instancePath); err == nil {
		var inst Instance
		if err := json.Unmarshal(data, &inst); err != nil {
			return nil, fmt.Errorf("unmarshal instance: %w", err)
		}
		return &inst, nil
	}

	inst := &Instance{
		ID:        uuid.New().String(),
		Name:      "openhub-instance",
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(inst, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal instance: %w", err)
	}

	if err := os.WriteFile(instancePath, data, 0600); err != nil {
		return nil, fmt.Errorf("write instance: %w", err)
	}

	return inst, nil
}
