package env

import (
	"github.com/cockroachdb/errors"
	"github.com/spf13/viper"
	"log"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	cmdDir    = "cmd"
	configDir = "configs"
)

// Read は環境変数とYAMLファイルから新規のコンフィグを取得
func Read(config any) {
	appEnv, err := GetAppEnv()
	if err != nil {
		log.Fatalf("get appEnv error: %s \n", err)
		return
	}
	if err := read(config, appEnv, getConfigDirPath(2)); err != nil {
		log.Fatalf("get config error: %s \n", err)
		return
	}
}

// ReadWithConfigDirPath は環境変数と指定の設定ディレクトリ名とYAMLファイルから新規のコンフィグを取得
func ReadWithConfigDirPath(config any, cfgDirPath string) {
	appEnv, err := GetAppEnv()

	if err != nil {
		log.Fatalf("get appEnv error: %s \n", err)
		return
	}
	if err := read(config, appEnv, cfgDirPath); err != nil {
		log.Fatalf("get config error: %s \n", err)
		return
	}
}

// read はconfigの読み込みを実施
func read(cfg any, cfgName string, cfgDirPath string) error {
	v := viper.New()
	v.AutomaticEnv()

	v.SetConfigName(cfgName)
	v.SetConfigType("yaml")
	v.AddConfigPath(cfgDirPath)

	if err := v.ReadInConfig(); err != nil {
		return errors.Errorf("read cfg error: %w", err)
	}
	if err := v.Unmarshal(cfg); err != nil {
		return errors.Errorf("parse cfg error: %w", err)
	}
	return nil
}

// getConfigDirPath configディレクトリの取得(readでのみ使用)
func getConfigDirPath(skip int) string {
	// クロスプラットフォーム対策
	_, file, _, _ := runtime.Caller(skip)
	dirList := strings.Split(filepath.ToSlash(filepath.Dir(file)), "/")
	dirPath := "./"

	for i, dir := range dirList {
		if dir == cmdDir {
			dirPath = filepath.Join(configDir, filepath.Join(dirList[i+1:]...))
			break
		}
	}
	return dirPath
}
