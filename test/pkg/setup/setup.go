package setup

import (
	"fmt"
	"os"
)

func RequireEnvs(keys ...string) error {
	missingEnvs := []string{}
	for _, key := range keys {
		_, err := GetRequiredEnv(key)
		if err != nil {
			missingEnvs = append(missingEnvs, key)
		}
	}
	if len(missingEnvs) > 0 {
		return fmt.Errorf("missing required env variable(s): %v, cannot continue", missingEnvs)
	}
	return nil
}

func GetRequiredEnv(key string) (string, error) {
	env := os.Getenv(key)
	if env == "" {
		return "", fmt.Errorf("\"%s\" env variable is required, cannot continue", key)
	}
	return env, nil
}
