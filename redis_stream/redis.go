package redis_stream

import (
	"context"
	"errors"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	redisCmdXAdd  = "XADD"
	redisCmdXRead = "XREAD"
)

var (
	NoTicketKeyErr  = errors.New("missing ticket key")
	NoTicketDataErr = errors.New("no ticket data")
	NoAssignmentErr = errors.New("missing assignment")
	InvalidInputErr = errors.New("invalid input")
)

type RedisConfig struct {
	OmCacheTicketTtlMs               int64 // チケットのキャッシュ生存期間
	OmCacheAssignmentAdditionalTtlMs int64 // チケットの有効期限切れ後、割り当て（assignment）情報をキャッシュに残しておく追加の時間（ミリ秒）
	Host                             string
	Port                             string
	OmRedisReadHost                  string
	OmRedisReadPort                  string
	OmRedisWriteHost                 string
	OmRedisWritePort                 string
	OmRedisPoolMaxIdle               int           // コネクションプール内でアイドル（未使用）のまま保持しておく最大接続数
	OmRedisPoolMaxActive             int           // コネクションプールから同時に貸し出される（利用中となる）最大接続数
	OmRedisPoolIdleTimeout           time.Duration // アイドル（未使用）状態がこの値（時間）を越えた接続は、自動的にクローズされる

	OmRedisReadUser      string
	OmRedisReadPassword  string
	OmRedisWriteUser     string
	OmRedisWritePassword string

	OmRedisUseTls                bool
	OmRedisDialMaxBackoffTimeout time.Duration
	OmRedisTlsSkipVerify         bool

	OmCacheInMaxUpdatesPerPoll             int // GetUpdate で一度に取得する最大更新数
	OmCacheInWaitTimeoutMs                 int // GetUpdate でストリームの更新待ち時のタイムアウト（GetUpdatesは非同期で実行されるため、実行をブロックしない）
	OmCacheOutWaitTimeoutMs                int // OutgoingReplicationQueue でリクエスト収集のタイムアウト
	OmCacheOutMaxQueueThreshold            int // OutgoingReplicationQueue でRedis にリクエストする処理要求のキューの最大値
	OmCacheInSleepBetweenApplyingUpdatesMs int // OutgoingReplicationQueue でキャッシュへの更新適用間のスリープ時間（ミリ秒単位）
}

type redisReplicator struct {
	rConnPool       *redis.Pool
	wConnPool       *redis.Pool
	cfg             *RedisConfig
	replId          string
	replIdValidator *regexp.Regexp
}

func NewRedis(config *RedisConfig) (*redisReplicator, error) {
	// シグナル通知
	ctx, cancel := context.WithCancel(context.Background())
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)

	var err error

	// 設定された有効期限より新しいすべての更新をリクエスト
	initialReplId := strconv.FormatInt(time.Now().UnixMilli()-config.OmCacheTicketTtlMs-config.OmCacheAssignmentAdditionalTtlMs, 10)

	// Redis Read URL
	readRedisHost := config.OmRedisReadHost
	readRedisPort := config.OmRedisReadPort
	readRedisUrl := fmt.Sprintf("%s:%s", readRedisHost, readRedisPort)

	// Redis Write URL
	writeRedisHost := config.OmRedisWriteHost
	writeRedisPort := config.OmRedisWritePort
	writeRedisUrl := fmt.Sprintf("%s:%s", writeRedisHost, writeRedisPort)

	rConnPool := getReadConnectionPool(ctx, *config, cancel, signalChan, readRedisUrl)
	wConnPool := getWriteConnectionPool(ctx, *config, cancel, signalChan, writeRedisUrl)

	rr := &redisReplicator{
		replIdValidator: regexp.MustCompile(`^\d{13}-\d+$`),
		replId:          initialReplId,
		cfg:             config,
		rConnPool:       rConnPool,
		wConnPool:       wConnPool,
	}

	// ReadRedisプールから接続を取得。内部でDial（新規接続）できるかどうかを確認。
	rConn, err := rr.rConnPool.GetContext(context.Background())
	if err == nil {
		// https://github.com/gomodule/redigo/blob/247f6c0e0a0ea200f727a5280d0d55f6bce6d2e7/redis/pool.go#L204
		// 接続をプールに戻す。
		defer rConn.Close()
	} else {
		//rConnLogger.WithFields(logrus.Fields{
		//	"error": err,
		//}).Debug("read redis connection error")
		return nil, err
	}

	// WriteRedisプールから接続を取得。内部でDial（新規接続）できるかどうかを確認。
	wConn, err := rr.wConnPool.GetContext(context.Background())
	if err == nil {
		defer wConn.Close()
	} else {
		//rConnLogger.WithFields(logrus.Fields{"error": err,}).Debug("write redis connection error")
		return nil, err
	}

	// コンテキストがSIGINTまたはSIGTERMによってキャンセルされたかどうかを確認。
	if ctx.Err() != nil {
		//rConnLogger.Fatal("cancellation requested")
		return nil, ctx.Err()
	}

	return rr, err
}

// getReadConnectionPool 読み取り専用のRedis接続プールの取得
func getReadConnectionPool(ctx context.Context, config RedisConfig, cancel context.CancelFunc, sigChan chan os.Signal, readRedisUrl string) *redis.Pool {
	return &redis.Pool{ // Redis read pool
		MaxIdle:     config.OmRedisPoolMaxIdle,
		MaxActive:   config.OmRedisPoolMaxActive,
		IdleTimeout: config.OmRedisPoolIdleTimeout,
		Wait:        true, // MaxActiveが上限値の場合、Get() は 空き（返却）接続が出るまで待つ（ブロックする）、falseの場合エラーになる。
		// アプリケーションが接続を再利用する前にアイドル状態の接続の健全性を確認するためのオプションのアプリケーション関数。関数がエラーを返した場合、接続は閉じられます。
		TestOnBorrow: func(c redis.Conn, lastUsed time.Time) error {
			// 15秒以内に使用された場合、接続は有効であるとみなす。
			if time.Since(lastUsed) < 15*time.Second {
				return nil
			}

			_, err := c.Do("PING")
			return err
		},
		// 接続の作成と設定を行うために提供されるアプリケーション関数
		Dial: func() (redis.Conn, error) {
			// https://cloud.google.com/memorystore/docs/redis/general-best-practices#operations_and_scenarios_that_require_a_connection_retry
			// https://pkg.go.dev/github.com/cenkalti/backoff/v4 パッケージを使い、
			// リトライの際にジッター付き指数バックオフを実装します。
			// 上限のタイムアウトに達するまでリトライを繰り返します。
			// リトライが成功するか、最終リトライが失敗した場合にのみ、Dial関数は返ります。
			var conn redis.Conn
			err := backoff.RetryNotify(
				func() error {

					// Local closure var
					var err error

					select {
					case <-sigChan:
						cancel()
					default:
						//rConnLogger.Debug("dialing Redis read replica")

						// Dial options
						dialOptions := []redis.DialOption{
							redis.DialUsername(config.OmRedisReadUser),
							redis.DialPassword(config.OmRedisReadPassword),
							redis.DialConnectTimeout(config.OmRedisPoolIdleTimeout), // Redis へ TCP 接続するまでの待ち時間の上限。
							redis.DialReadTimeout(config.OmRedisPoolIdleTimeout),    // Redis にコマンドを送った後、レスポンスを読み取る待ち時間の上限。
						}

						// TLSを使用するオプションの追加
						if config.OmRedisUseTls {
							//rConnLogger.Info("OM_REDIS_USE_TLS is set to true, will attempt to connect to Redis read replica(s) using TLS.")
							dialOptions = append(dialOptions, redis.DialUseTLS(true))
						}

						// 設定フラグが設定されている場合、TLS証明書の検証をスキップする (例: 自己署名証明書の場合)
						if config.OmRedisTlsSkipVerify {
							//rConnLogger.Info("OM_REDIS_TLS_SKIP_VERIFY is set to true, will attempt to connect to Redis read replica(s) using TLS without verifying the TLS certificate.")
							dialOptions = append(dialOptions, redis.DialTLSSkipVerify(true))
						}

						// redisへ接続
						conn, err = redis.Dial("tcp",
							readRedisUrl,
							dialOptions...,
						)
						if err != nil { // Check for error on dial
							//rConnLogger.Error("failure dialing Redis read replica")
						}
					}
					return err
				},
				backoff.WithContext(
					backoff.NewExponentialBackOff(
						backoff.WithMaxElapsedTime(config.OmRedisDialMaxBackoffTimeout)), ctx), // リトライを粘る合計時間の上限を設定
				func(err error, bo time.Duration) {
					//rConnLogger.WithFields(logrus.Fields{"error": err}).Debugf(
					//	"Error attempting to connect to Redis read replica. Retrying in %s", bo)
				},
			)
			return conn, err
		},
	}
}

// getWriteConnectionPool 書き込み専用のRedis接続プールの取得
func getWriteConnectionPool(ctx context.Context, config RedisConfig, cancel context.CancelFunc, sigChan chan os.Signal, readRedisUrl string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     config.OmRedisPoolMaxIdle,
		MaxActive:   config.OmRedisPoolMaxActive,
		IdleTimeout: config.OmRedisPoolIdleTimeout,
		Wait:        true,
		TestOnBorrow: func(c redis.Conn, lastUsed time.Time) error {
			if time.Since(lastUsed) < 15*time.Second {
				return nil
			}

			_, err := c.Do("PING")
			return err
		},
		Dial: func() (redis.Conn, error) {
			var conn redis.Conn
			err := backoff.RetryNotify(
				func() error {

					// Local closure var
					var err error

					select {
					case <-sigChan:
						cancel()
					default:
						// Dial options
						dialOptions := []redis.DialOption{
							redis.DialPassword(config.OmRedisWritePassword),
							redis.DialConnectTimeout(config.OmRedisPoolIdleTimeout),
							redis.DialReadTimeout(config.OmRedisPoolIdleTimeout),
						}

						if config.OmRedisUseTls {
							//rConnLogger.Info("OM_REDIS_USE_TLS is set to true, will attempt to connect to Redis read replica(s) using TLS.")
							dialOptions = append(dialOptions, redis.DialUseTLS(true))
						}

						if config.OmRedisTlsSkipVerify {
							//rConnLogger.Info("OM_REDIS_TLS_SKIP_VERIFY is set to true, will attempt to connect to Redis read replica(s) using TLS without verifying the TLS certificate.")
							dialOptions = append(dialOptions, redis.DialTLSSkipVerify(true))
						}

						conn, err = redis.Dial("tcp",
							readRedisUrl,
							dialOptions...,
						)
						if err != nil {
							//rConnLogger.Error("failure dialing Redis read replica")
						}

					}
					return err
				},
				backoff.WithContext(backoff.NewExponentialBackOff(backoff.WithMaxElapsedTime(config.OmRedisDialMaxBackoffTimeout)), ctx),
				func(err error, bo time.Duration) {
					//rConnLogger.WithFields(logrus.Fields{"error": err}).Debugf(
					//	"Error attempting to connect to Redis read replica. Retrying in %s", bo)
				},
			)
			return conn, err
		},
	}
}

// SendUpdates は状態更新構造体の配列を受け取り、それらをデータストレージに書き込む。
// これにより、すべてのクライアント（例：他の om-core インスタンス）に複製されます。
// Redisを使用する場合、パフォーマンス向上のためこれらの更新はバッチ更新としてパイプライン処理されます。
// 関数に送信される更新に加え、各バッチの終了時には必ずXTRIMコマンドを実行し、
// 環境変数OM_CACHE_TICKET_TTL_MSで設定されたTTLより古い更新を全て削除します。
func (rr *redisReplicator) SendUpdates(updates []*StateUpdate) []*StateResponse {
	logger := logrus.WithFields(logrus.Fields{
		"app":       "open_match",
		"component": "redisReplicator.sendUpdates",
	})

	// Var init
	var err error
	out := make([]*StateResponse, len(updates))

	// WritePoolから接続情報を取得
	rConn := rr.wConnPool.Get()
	defer rConn.Close()

	// ====== XADD ======
	// 要求されたすべてのRedisコマンドを処理
	for i, update := range updates {
		out[i] = &StateResponse{Result: "", Err: nil}
		redisArgs := make([]interface{}, 0)
		redisArgs = append(redisArgs, "om-replication", "*")

		switch update.Cmd {
		case Ticket:
			// Validate input
			if update.Value == "" {
				out[i].Err = NoTicketDataErr
				continue
			}
			redisArgs = append(redisArgs, "ticket")
			redisArgs = append(redisArgs, update.Value)
		case Activate:
			// Validate input
			if update.Key == "" {
				out[i].Err = NoTicketKeyErr
				continue
			}
			redisArgs = append(redisArgs, "activate")
			redisArgs = append(redisArgs, update.Key)
		case Deactivate:
			// Validate input
			if update.Key == "" {
				out[i].Err = NoTicketKeyErr
				continue
			}
			redisArgs = append(redisArgs, "deactivate")
			redisArgs = append(redisArgs, update.Key)
		case Assign:
			// TODO: 1つのRedisレコードに複数の割り当てを格納するかどうかを決定
			// 現在、Redisストリームの各エントリは1つの割り当てのみ保持する
			// 単一エントリに複数の割り当てを格納することは可能（キー/値ペアの数は任意）だが、
			// 割り当ては非推奨のため、他の課題に集中するためこの最適化は保留中
			if update.Key == "" {
				out[i].Err = NoTicketKeyErr
				continue
			}
			if update.Value == "" {
				out[i].Err = NoAssignmentErr
				continue
			}
			redisArgs = append(redisArgs, "assign")
			redisArgs = append(redisArgs, update.Key)
			redisArgs = append(redisArgs, "connection")
			redisArgs = append(redisArgs, update.Value)
		default:
			// 不明瞭な重大問題が発生した場合
			out[i].Err = InvalidInputErr
			continue
		}

		// 構築されたredisコマンド
		redisCmdWithArgs := fmt.Sprintf("%v %v", redisCmdXAdd, strings.Trim(fmt.Sprint(redisArgs), "[]"))
		logger.Debug(redisCmdWithArgs)

		// コマンドをバッファに追加
		err = rConn.Send(redisCmdXAdd, redisArgs...)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"redis_command": redisCmdWithArgs,
			}).Errorf("Redis error: %v", err)
		}
	}

	// ====== XTRIM ======
	// 期限切れのエントリを削除する追加コマンドをパイプラインに追加
	expirationThresh := strconv.FormatInt(time.Now().UnixMilli()-rr.cfg.OmCacheTicketTtlMs, 10)
	redisCmdWithArgs := fmt.Sprintf("XTRIM om-replication MINID %v", expirationThresh)
	logger.Debug(redisCmdWithArgs)

	// XTRIMコマンドをバッファに追加
	err = rConn.Send("XTRIM", "om-replication", "MINID", expirationThresh)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"redis_command": redisCmdWithArgs,
		}).Errorf("Redis error: %v", err)
	}

	// パイプライン化されたコマンドを送信し、結果を取得
	// 第一引数か空文字の場合はパイプラインの返信回収用になる。
	r, err := rConn.Do("")
	if err != nil {
		logger.Errorf("Redis error when executing batch: %v", err)
	}

	// コマンド実行結果の存在確認
	if r == nil {
		logger.Error("Redis returned empty results from update!")
		return out
	}

	// 結果
	// r = [
	//  XADD(update[0])の結果,
	//  XADD(update[1])の結果,
	//  ...
	//  XADD(update[n-1])の結果,
	//  XTRIMの結果(削除件数)   // ←最後
	//]

	// コマンド実行結果の最終要素 を取り出して、int64 に変換。
	// Redisからの最終結果は、XTRIMコマンドで削除したエントリ数のカウントになる。
	expiredCount, err := redis.Int64(r.([]interface{})[len(r.([]interface{}))-1], err)
	if err != nil {
		logger.Errorf("Redis output int64 conversion error: %v", err)
	}
	if expiredCount > 0 {
		logger.WithFields(logrus.Fields{
			"redis_expiration_count":  expiredCount,
			"expiration_threshold_ts": expirationThresh,
		}).Debug("Expired Redis entries older than expiration threshold timestamp")
	}

	// その他のすべてのRedis結果を返り値の出力配列に処理
	// 最終はXTRIMの削除件数が入っているので、-1している。
	for index := 0; index < len(r.([]interface{}))-1; index++ {
		// 更新の解析で問題が見つからなかったことを確認してください。
		// 更新の解析でエラーが発生した場合、更新に必要なフィールドがすべて揃っていなかったため、Redisには送信されませんでした。
		if out[index].Err != nil {
			logger.WithFields(logrus.Fields{"update": updates[index]}).Error("an update could not be parsed and was skipped")

			// Redisの結果を返す必要がないため、更新の解析エラーとエラーを発生させたキーを返す。
			out[index].Result = updates[index].Key
		} else {
			// 更新が正常な場合
			t, err := redis.String(r.([]interface{})[index], err)
			if err != nil {
				// Redisの結果が文字列ではない場合はエラーになる。エラーコードを返し、結果はエラーを発生させたキーになる。
				t = updates[index].Key
				out[index].Err = fmt.Errorf("Redis output string conversion error: %w", err)
				logger.WithFields(logrus.Fields{"err": err, "update": updates[index]}).Error("Redis returned an error while trying to update")
			}

			logger.WithFields(logrus.Fields{"update": updates[index], "result": t}).Tracef("Redis successfully processed update")
			out[index].Result = t
		}
	}

	return out
}

// GetUpdates は状態に対してブロッキング読み取りを実行し（OmCacheInWaitTimeoutMs から読み取る設定可能なタイムアウト付き）、
// 前回の GetUpdates リクエスト以降にレプリケーターに送信されたすべての更新構造体を受信します。
// これらは配列で返され、om-core はそれらを 元の受信順序でイベントとして適用します。
func (rr *redisReplicator) GetUpdates() []*StateUpdate {
	logger := logrus.WithFields(logrus.Fields{
		"app":       "open_match",
		"component": "redisReplicator.getUpdates",
	})

	out := make([]*StateUpdate, 0)

	// 更新を取得するためのredisコマンド作成
	redisArgs := make([]interface{}, 0)
	//  一度に取得する最大更新数
	redisArgs = append(redisArgs, "COUNT", rr.cfg.OmCacheInMaxUpdatesPerPoll)

	// 指定したミリ秒の間 新しいデータが無ければ接続を待機状態にする。
	redisArgs = append(redisArgs, "BLOCK", rr.cfg.OmCacheInWaitTimeoutMs)

	// 取得対象のストリーム名
	redisArgs = append(redisArgs, "STREAMS", "om-replication")

	// このストリームから読み取られた最終ID
	redisArgs = append(redisArgs, rr.replId)

	// 読み取りのプールから接続情報取得
	rConn := rr.rConnPool.Get()
	defer rConn.Close()

	// Execute XREAD
	logger.WithFields(logrus.Fields{
		"redisCmd": fmt.Sprint(redisCmdXRead, strings.Trim(fmt.Sprint(redisArgs), "[]")),
	}).Debugf("Executing redis command")
	data, err := rConn.Do(redisCmdXRead, redisArgs...)
	if err != nil {
		logger.Errorf("Redis error: %v", err)
	}

	// Redigoモジュールは、タイムアウト（BLOCK Xms）に達するまでに更新を確認できなかった場合、
	// データに対してnilを返すので、その際は単に正常に返却してください。
	if data != nil {
		switch data.(type) {
		case redis.Error:
			logger.Errorf("Redis error: %v", data.(redis.Error))
		case []interface{}:
			// データはレスポンス内で数段階ネストされた配列レベルにある
			// https://redis.io/docs/latest/develop/data-types/streams/#listening-for-new-items-with-xread
			replStream := data.([]interface{})[0].([]interface{})[1].([]interface{})
			for _, v := range replStream {
				// 要素0はRedisのストリームエントリIDであり、これをレプリケーションIDとして使用
				replId, err := redis.String(v.([]interface{})[0], nil)
				if err != nil {
					logger.Error(err)
				}
				thisUpdate := &StateUpdate{}

				// 要素1はストリームエントリ内の実際のデータ。
				// 各ストリームエントリに1つの更新のみが保存される想定（Redisは複数許可可能）。
				y, err := redis.Strings(v.([]interface{})[1], nil)
				if err != nil {
					logger.Error(err)
				}

				// Update type/key/value data
				switch y[0] {
				case "ticket":
					thisUpdate.Cmd = Ticket
					thisUpdate.Key = replId
					thisUpdate.Value = y[1] // Only argument for a ticket is the ticket PB
				case "activate":
					thisUpdate.Cmd = Activate
					thisUpdate.Key = y[1] // チケットの有効化に必要な引数は、チケットのIDのみ
				case "deactivate":
					thisUpdate.Cmd = Deactivate
					thisUpdate.Key = y[1] // チケットの無効化に必要な引数は、チケットのIDのみ
				case "assign":
					// XADD om-replication * assign ticket-123 connection conn-A
					// 127.0.0.1:16379> XREAD STREAMS om-replication  0-0
					// 1) 1) "om-replication"
					// 2) 1) 1) "1765728634700-0"
					//    2) 1) "assign"
					//       2) "ticket-123"
					//       3) "connection"
					//       4) "conn-A"

					thisUpdate.Cmd = Assign
					thisUpdate.Key = y[1]   // ticket's ID
					thisUpdate.Value = y[3] // assignment
				}

				out = append(out, thisUpdate)

				// 現在の replId を更新し、この更新が処理されたことを示す
				rr.replId = replId
			}
		}
	}
	return out
}

// GetReplIdValidator は、文字列が有効なレプリケーション ID（Redis ストリームエントリ ID）の形式であるかどうかを
// 検証するために使用できるコンパイル済み正規表現を返します。
func (rr *redisReplicator) GetReplIdValidator() *regexp.Regexp {
	return rr.replIdValidator
}
