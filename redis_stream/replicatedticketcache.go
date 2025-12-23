package redis_stream

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	pb "github.com/googleforgames/open-match2/v2/pkg/pb"

	"github.com/sirupsen/logrus"
)

var (
	logger = logrus.WithFields(logrus.Fields{
		"app":       "open_match",
		"component": "cache",
	})
)

// UpdateRequest は基本的に、StateUpdate（store.go）をラップしたもので、コンテキストと結果を受け取るチャネルを追加します。
// 状態ストレージ層は、基盤となるコンテキストや更新リクエストの発信元（つまりリクエストが戻ってくる場所）を理解する必要はありません。
// ただし、これらは om-core 内の gRPC サーバーには必要です！
type UpdateRequest struct {
	Ctx context.Context
	// The update itself.
	Update StateUpdate
	// チケットIDを返送することで書き込みを確認するための返信チャネル
	ResultsChan chan *StateResponse
}

// ReplicatedTicketCache サーバーは起動時にインスタンス化される構造体。
// すべてのチケット読み取り機能はこのローカルキャッシュから読み取ります。
// このキャッシュには、gRPC呼び出しを処理することでこのインスタンスに届くチケットキャッシュ変更を複製するために必要なデータ構造も備わっています。
// このデータ構造はsync.Mapメンバーを含むため、インスタンス化後はコピーすべきではありません（詳細はhttps://pkg.go.dev/sync#Mapを参照）。
type ReplicatedTicketCache struct {
	// すべての状態データのローカルコピー
	Tickets     sync.Map
	InactiveSet sync.Map
	Assignments sync.Map

	// この replicatedTicketCache がどのように複製されるか。
	Replicator StateReplicator
	// キャッシュ更新のキュー
	UpRequests chan *UpdateRequest

	IdValidator *regexp.Regexp

	Cfg *RedisConfig
}

// OutgoingReplicationQueue はサーバーの存続期間中実行される非同期ゴルーチン。
// gRPC ハンドラーによって生成される受信レプリケーションイベントを処理し、
// それらのイベントを構成済みの状態ストレージに送信します。この時点では更新はまだチケットキャッシュのローカルコピーに適用されていません。
// イベントが正常にレプリケーションされ、incomingReplicationQueue ゴルーチンで受信されると、更新がローカルキャッシュに適用されます。
func (tc *ReplicatedTicketCache) OutgoingReplicationQueue(ctx context.Context) {
	logger := logger.WithFields(logrus.Fields{
		"app":       "open_match",
		"component": "replicationQueue",
		"direction": "outgoing",
	})

	logger.Debug("Listening for replication requests")
	exec := false
	pipelineRequests := make([]*UpdateRequest, 0)
	pipeline := make([]*StateUpdate, 0)

	for {
		exec = false
		pipelineRequests = pipelineRequests[:0] // 前回の処理で追加された分の容量はそのままにして初期化することで、余計なメモリ確保を防ぐ
		pipeline = pipeline[:0]
		timeout := time.After(time.Millisecond * time.Duration(tc.Cfg.OmCacheOutWaitTimeoutMs))

		// 単一のコマンドで状態ストレージへの書き込みを待機中のリクエストを収集する（例：Redis Pipelining）
		for exec != true {
			select {
			case req := <-tc.UpRequests:
				pipelineRequests = append(pipelineRequests, req)
				pipeline = append(pipeline, &req.Update)

				logger.Tracef(" %v requests queued for current batch", len(pipelineRequests))
				// 最大バッチサイズに到達した場合
				if len(pipelineRequests) >= tc.Cfg.OmCacheOutMaxQueueThreshold {
					logger.Trace("OM_CACHE_OUT_MAX_QUEUE_THRESHOLD reached")
					exec = true
				}
			// タイムアウトの場合、バッチのキューを万杯まで待たない
			case <-timeout:
				//otelCacheOutgoingQueueTimeouts.Add(ctx, 1)
				logger.Trace("OM_CACHE_OUT_WAIT_TIMEOUT_MS reached")
				exec = true
			}
		}

		// Redisの更新パイプラインバッチジョブに実行すべきコマンドがある場合、実行
		if len(pipelineRequests) > 0 {
			// リクエスト数記録
			logger.WithFields(logrus.Fields{
				"batch_update_count": len(pipelineRequests),
			}).Trace("sending state update batch to replicator")
			//otelCacheOutgoingUpdatesPerPoll.Record(ctx, int64(len(pipelineRequests)))

			// 更新のバッチをRedisへ書き込み
			results := tc.Replicator.SendUpdates(pipeline)

			// レプリケーターから受信した結果の数を記録
			logger.WithFields(logrus.Fields{
				"result_count": len(results),
			}).Trace("state update batch results received from replicator")

			for index, result := range results {
				// リクエストしたチャネルに結果を送信
				pipelineRequests[index].ResultsChan <- result
			}
		}
	}
}

// IncomingReplicationQueue はサーバーの存続期間中実行される非同期ゴルーチンです。
// 設定された状態ストレージから受信レプリケーションイベントを全て読み取り、 ローカルチケットキャッシュに適用します。
// 実際には、InvokeMatchMakingFunction を除き、全ての om-core gRPC ハンドラーの
// ほぼ全ての処理をこのゴルーチンが担います。
func (tc *ReplicatedTicketCache) IncomingReplicationQueue(ctx context.Context) {
	logger := logger.WithFields(logrus.Fields{
		"app":       "open_match",
		"component": "replicationQueue",
		"direction": "incoming",
	})

	// Redisのレプリケーションストリームを非同期で監視し、
	// 更新データをチャンネルに追加して、到着順に処理されるようにする
	replStream := make(chan StateUpdate, tc.Cfg.OmCacheInMaxUpdatesPerPoll)
	go func() {
		for {
			// GetUpdates() コマンドは更新を検知すると直ちに返ります。
			// 更新処理は OmCacheInWaitTimeoutMs ミリ秒ごとに一度だけ実行したいので、
			// 期限を設定し、期限切れ後にのみループを実行します。これを設定しないと、例えば数ミリ秒ごとに1つずつしか
			// 流入しない更新のような特定のケースでは、
			// ループが高速に繰り返され、各処理でわずかな作業量しか 完了できなくなります。
			deadline := time.After(time.Millisecond * time.Duration(tc.Cfg.OmCacheInWaitTimeoutMs))

			// GetUpdates()は更新がない場合にブロックするが、
			// 内部実装では設定変数OM_CACHE_IN_WAIT_TIMEOUT_MSで定義されたタイムアウトを遵守するため、
			// タイムリーな返却が保証される。保留中の更新が最大 OmCacheInMaxUpdatesPerPoll 個存在する場合、その数まで取得します。
			results := tc.Replicator.GetUpdates()

			//otelCacheIncomingPerPoll.Record(ctx, int64(len(results)))

			if len(results) == 0 {
				//otelCacheIncomingEmptyTimeouts.Add(ctx, 1)
			}

			// 更新内容をレプリケーションチャネルに投入して処理
			for _, curUpdate := range results {
				logger.WithFields(logrus.Fields{
					"update.key":     curUpdate.Key,
					"update.command": curUpdate.Cmd,
				}).Trace("queueing incoming update from state storage")
				replStream <- *curUpdate
			}

			// OmCacheInWaitTimeoutMs ミリ秒が経過したことを確認してから、 次の更新を取得しようと試みます。
			<-deadline
		}
	}()

	// チャンネルの更新を確認し、適用する
	for {
		// タイトなループと高いCPU使用率を回避するため。レプリケーション更新をローカルキャッシュに適用する間の強制スリープ時間
		time.Sleep(time.Millisecond * time.Duration(tc.Cfg.OmCacheInSleepBetweenApplyingUpdatesMs))
		done := false

		var err error
		for !done {
			// 更新処理を実行できる最大時間。更新中はチケットキャッシュへのアクセスがロックされるため、
			// 無限のミューテックスロックや競合状態を回避するために、ここに厳密な制限を設ける必要がある
			updateTimeout := time.After(time.Millisecond * 500)

			// 残りの更新がなくなるかロックタイムアウトに達するまで、 すべての受信更新を処理する。
			select {
			case curUpdate := <-replStream:
				switch curUpdate.Cmd {
				case Ticket:
					// 更新値をプロトバフメッセージに変換し、 保存する。
					//
					// https://protobuf.dev/best-practices/dos-donts/#separate-types-for-storage
					// では、これは推奨されないパターンであると述べられていますが、om-coreは例外となる条件を満たしています：
					// 「以下のすべてが真である場合：
					//
					// - あなたのサービスがストレージシステムであること
					// - あなたのシステムがクライアントの構造化データに基づいて 判断を行わないこと
					// - あなたのシステムが単に保存、読み込み、そしておそらく ライアントの要求に応じてクエリを提供するだけであること
					ticketPb := &pb.Ticket{}
					err = proto.Unmarshal([]byte(curUpdate.Value), ticketPb)
					if err != nil {
						logger.Error("received ticket replication could not be unmarshalled")
					}

					// TicketIDを設定する。レプリケーション後に実行する必要がある。
					// TicketID自体がレプリケーションID（例：RedisストリームエントリID）であるため
					// これにより、クライアントが正常に保存/レプリケートされなかった無効なticketIDを取得する可能性を完全に排除できる
					ticketPb.Id = curUpdate.Key

					// すべてのチケットは非アクティブ状態で開始
					tc.InactiveSet.Store(curUpdate.Key, true)
					tc.Tickets.Store(curUpdate.Key, ticketPb)
					logger.Tracef("ticket replication received: %v", curUpdate.Key)

				case Activate:
					tc.InactiveSet.Delete(curUpdate.Key)
					logger.Tracef("activation replication received: %v", curUpdate.Key)

				case Deactivate:
					tc.InactiveSet.Store(curUpdate.Key, true)
					logger.Tracef("deactivate replication received: %v", curUpdate.Key)

				case Assign:
					// protobuf messageに更新
					assignmentPb := &pb.Assignment{}
					err = proto.Unmarshal([]byte(curUpdate.Value), assignmentPb)
					if err != nil {
						logger.Error("received assignment replication could not be unmarshalled")
					}
					tc.Assignments.Store(curUpdate.Key, assignmentPb)
					logger.Tracef("**DEPRECATED** assign replication received %v:%v", curUpdate.Key, assignmentPb.GetConnection())
				}
			case <-updateTimeout:
				//otelCacheIncomingProcessingTimeouts.Add(ctx, 1)
				logger.Trace("lock hold timeout")
				done = true
			default:
				logger.Trace("Incoming update queue empty")
				done = true
			}
		}

		// Expiration closure, contains all code that removes data from the
		// replicated ticket cache.
		//
		// No need for this to be it's own function yet as the performance is
		// satisfactory running it immediately after every cache update.
		//
		// Removal logic is as follows:
		// * ticket ids expired from the inactive list MUST also have their
		//   tickets removed from the ticket cache! Any ticket that exists and
		//   doesn't have it's id on the inactive list is considered active and
		//   will appear in ticket pools for invoked MMFs!
		// * tickets with user-specified expiration times sooner than the
		//   default MUST be removed from the cache at the user-specified time.
		//   Inactive list is not affected by this as inactive list entries for
		//   tickets that don't exist have no effect (except briefly taking up a
		//   few bytes of memory). Dangling inactive list entries will be cleaned
		//   up in expirations cycles after the configured OM ticket TTL anyway.
		// * assignments are expired after the configured OM ticket TTL AND the
		//   configured OM assignment TTL have elapsed. This is to handle cases
		//   where tickets expire after they were passed to invoked MMFs but
		//   before they are in sessions. Such tickets are still allowed to be
		//   assigned and their assignments will be retained until the
		//   expiration time described above. **DEPRECATED**
		{
			// Separate logrus instance with its own metadata to aid troubleshooting
			exLogger := logrus.WithFields(logrus.Fields{
				"app":       "open_match",
				"component": "replicatedTicketCache",
				"operation": "expiration",
			})

			var (
				numInactive            int64
				numInactiveDeletions   int64
				numTickets             int64
				numTicketDeletions     int64
				numAssignments         int64
				numAssignmentDeletions int64
			)
			startTime := time.Now()

			// ローカルキャッシュの非アクティブチケットセットから期限切れチケットを削除する。
			// この期限切れ処理は、チケットがシステムに投入された時刻と設定された最大チケットTTLに基づいて行われ、
			// チケットの非アクティブ状態が作成された時刻に基づくものではない。これはつまり、
			// ユーザー設定の有効期限により既に期限切れとなったチケットが、
			// 短時間だけ非アクティブセット内に残存する可能性があることを意味します。
			// 存在しないチケットを参照する非アクティブセットのエントリは影響を与えないため、これは問題ありません。
			tc.InactiveSet.Range(func(id, _ any) bool {
				// IDから作成タイムスタンプを取得。タイムスタンプは常に RedisストリームエントリIDの規約に従い
				// ミリ秒精度（13桁のUnixタイムスタンプ）であると仮定される
				ticketCreationTime, err := strconv.ParseInt(strings.Split(id.(string), "-")[0], 10, 64)
				if err != nil {
					exLogger.WithFields(logrus.Fields{
						"ticket_id": id,
					}).Error("Unable to parse ticket ID into an unix timestamp when trying to expire old ticket active states")
					return true
				}
				if (time.Now().UnixMilli() - ticketCreationTime) > int64(tc.Cfg.OmCacheTicketTtlMs) {
					// 非アクティブセットからチケットを期限切れにする際、そのチケットが常に削除されることを保証する。
					_, existed := tc.Tickets.LoadAndDelete(id)
					if existed {
						numTickets++
						numTicketDeletions++
					}

					// 無効なチケットを非アクティブセットから削除する。
					tc.InactiveSet.Delete(id)
					numInactiveDeletions++
				} else {
					numInactive++
				}
				return true
			})

			// チケットの有効期限に基づいて、ローカルキャッシュから期限切れのチケットを削除する。
			tc.Tickets.Range(func(id, ticket any) bool {
				if time.Now().After(ticket.(*pb.Ticket).GetExpirationTime().AsTime()) {
					tc.Tickets.Delete(id)
					numTicketDeletions++
				} else {
					numTickets++
				}
				return true
			})

			// ローカルキャッシュから期限切れの割り当てを削除します。これは非アクティブセットからチケットを期限切れにするロジックと類似していますが、追加の構成可能なTTLを含みます。
			// これにより、チケットがシステムに存在しなくなった後も、割り当てが保持され取得されることが可能になります。
			// 割り当て追跡は非推奨機能であり、
			// マッチメイカーの迅速な反復処理中にのみ使用することを意図しています。
			// 本番環境では、より堅牢なプレイヤー状態追跡システムの使用を強く推奨します。
			tc.Assignments.Range(func(id, _ any) bool {
				// IDから作成時刻を取得する
				ticketCreationTime, err := strconv.ParseInt(strings.Split(id.(string), "-")[0], 10, 64)
				if err != nil {
					exLogger.Error("Unable to parse ticket ID into an unix timestamp when trying to expire old assignments")
				}
				if (time.Now().UnixMilli() - ticketCreationTime) >
					int64(tc.Cfg.OmCacheTicketTtlMs+tc.Cfg.OmCacheAssignmentAdditionalTtlMs) {
					tc.Assignments.Delete(id)
					numAssignmentDeletions++
				} else {
					numAssignments++
				}
				return true
			})

			// Log results and record counter metrics
			//AssignmentCount = numAssignments
			//TicketCount = numTickets
			//InactiveCount = numInactive

			// Record time elapsed for histogram
			elapsed := float64(time.Since(startTime).Microseconds())
			//otelCacheExpirationCycleDuration.Record(ctx, elapsed/1000.0) // OTEL measurements are in milliseconds

			// Record expiration count data for histograms
			//otelCacheTicketsExpiredPerCycle.Record(ctx, int64(numTicketDeletions))
			//otelCacheInactivesExpiredPerCycle.Record(ctx, int64(numInactiveDeletions))
			//otelCacheAssignmentsExpiredPerCycle.Record(ctx, int64(numAssignmentDeletions))

			// Trace logging for advanced debugging
			if numAssignmentDeletions > 0 {
				exLogger.Tracef("Removed %v expired assignments from local cache", numAssignmentDeletions)
			}
			if numInactiveDeletions > 0 {
				exLogger.Tracef("%v ticket ids expired from the inactive list in local cache", numInactiveDeletions)
			}
			if numTicketDeletions > 0 {
				exLogger.Tracef("Removed %v expired tickets from local cache", numTicketDeletions)
			}
			if elapsed >= 0.01 {
				exLogger.Tracef("Local cache expiration code took %.2f us", elapsed)
			}
		}
	}
}
