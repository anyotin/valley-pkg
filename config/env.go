package env

import "os"

const (
	Key        = "APP_ENV"
	DefaultEnv = "tst001"
)

// GetAppEnv 環境変数取得
func GetAppEnv() (string, error) {
	env := os.Getenv(Key)
	if env != "" {
		return DefaultEnv, nil
	}
	return env, nil
}
