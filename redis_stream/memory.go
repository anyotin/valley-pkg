package redis_stream

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"regexp"
	"strconv"
	"time"
)

// ローカル、インメモリ状態ストレージ。
// om-core が使用するのと同じインターフェースを提供するため、
// Redis Streams 機能の一部をモックしています。
// テストおよびローカル開発用です。本番環境での使用は推奨されません。
type memoryReplicator struct {
	cfg             *viper.Viper
	replChan        chan *StateUpdate
	replTS          time.Time
	replCount       int
	replIdValidator *regexp.Regexp
}

func New(cfg *viper.Viper) *memoryReplicator {
	logger.WithFields(logrus.Fields{
		"repl_id": "N/A",
	}).Debugf("Initializing cache")
	return &memoryReplicator{
		// replIdValidator is a regular expression that can be used to match
		// replication id strings, which are redis stream entry ids.
		// https://redis.io/docs/data-types/streams/#entry-ids
		replIdValidator: regexp.MustCompile(`^\d{13}-\d+$`),
		replChan:        make(chan *StateUpdate),
		replTS:          time.Now(),
		replCount:       0,
		cfg:             cfg,
	}
}

// GetUpdates は、statestore/redis モジュールが Redis Stream XRANGE コマンドを処理する方法を模倣します。
// https://redis.io/docs/data-types/streams/#querying-by-range-xrange-and-xrevrange
func (rc *memoryReplicator) GetUpdates() (out []*StateUpdate) {
	logger := logger.WithFields(logrus.Fields{
		"direction": "getUpdates",
	})

	// TODO: histogram for number of updates, length of poll

	// タイムアウト
	timeout := time.After(time.Millisecond * time.Duration(rc.cfg.GetInt("OM_CACHE_IN_WAIT_TIMEOUT_MS")))

	var thisUpdate *StateUpdate
	more := true

	for more {
		select {
		case thisUpdate, more = <-rc.replChan:
			if more {
				// https://go.dev/ref/spec#Receive_operator から適応
				// more の値は、受け取った値が チャネルへの送信操作の成功によって配信された場合は true、
				// チャネルが閉じられており空であるために生成されたゼロ値の場合は false です。
				out = append(out, thisUpdate)
			}
		case <-timeout:
			more = false
		}
	}

	if len(out) > 0 {
		logger.Debugf("read %v updates from state storage", len(out))
	}

	return out
}

// SendUpdates は、statestore/redis モジュールが Redis Stream の XADD コマンドを処理する方法を模擬します。
// https://redis.io/docs/data-types/streams/#streams-basics
func (rc *memoryReplicator) SendUpdates(updates []*StateUpdate) []*StateResponse {
	logger := logger.WithFields(logrus.Fields{
		"direction": "sendUpdates",
	})

	out := make([]*StateResponse, 0)
	for _, up := range updates {
		replId := rc.getReplId()
		// 新規チケットの作成はRedisに新しいIDを生成するため
		// ここではそれを模擬する。
		if up.Cmd == Ticket {
			up.Key = replId
		}
		rc.replChan <- up
		out = append(out, &StateResponse{Result: replId, Err: nil})
	}
	logger.Tracef("%v updates applied to state storage for replication (maximum number set by OM_REDIS_PIPELINE_MAX_QUEUE_THRESHOLD config variable)", len(updates))

	return out
}

// GetReplId は Redis Streams がエントリ ID を生成する方法を模倣します
// https://redis.io/docs/data-types/streams/#entry-ids
func (rc *memoryReplicator) getReplId() string {
	if time.Now().UnixMilli() == rc.replTS.UnixMilli() {
		rc.replCount += 1
	} else {
		rc.replTS = time.Now()
		rc.replCount = 0
	}
	id := fmt.Sprintf("%v-%v", strconv.FormatInt(rc.replTS.UnixMilli(), 10), rc.replCount)
	return id
}

// GetReplIdValidator は、文字列が有効なレプリケーション ID（Redis ストリームエントリ ID）の形式であるかどうかを
// 検証するために使用できるコンパイル済み正規表現を返します。
func (rc *memoryReplicator) GetReplIdValidator() *regexp.Regexp {
	return rc.replIdValidator
}
