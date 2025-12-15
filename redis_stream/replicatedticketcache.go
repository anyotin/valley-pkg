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
	// Return channel to confirm the write by sending back the assigned ticket ID
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
		pipelineRequests = pipelineRequests[:0]
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
				// Still updates to process.
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

					// Set TicketID. Must do it post-replication since
					// TicketID /is/ the Replication ID
					// (e.g. redis stream entry id)
					// This guarantees that the client can never get an
					// invalid ticketID that was not successfully stored/replicated
					ticketPb.Id = curUpdate.Key

					// All tickets begin inactive
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
					// Convert the assignment back into a protobuf message.
					assignmentPb := &pb.Assignment{}
					err = proto.Unmarshal([]byte(curUpdate.Value), assignmentPb)
					if err != nil {
						logger.Error("received assignment replication could not be unmarshalled")
					}
					tc.Assignments.Store(curUpdate.Key, assignmentPb)
					logger.Tracef("**DEPRECATED** assign replication received %v:%v", curUpdate.Key, assignmentPb.GetConnection())
				}
			case <-updateTimeout:
				// Lock hold timeout exceeded
				//otelCacheIncomingProcessingTimeouts.Add(ctx, 1)
				logger.Trace("lock hold timeout")
				done = true
			default:
				// Nothing left to process; exit immediately
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
		//
		// TODO: measure the impact of expiration operations with a timer
		// metric under extreme load, and if it's problematic,
		// revisit / possibly do it asynchronously
		//
		// Ticket IDs are assigned by Redis in the format of
		// <unix_timestamp>-<index> where the index increments every time a
		// ticket is created during the same second. We are only
		// interested in the ticket creation time (the unix timestamp) here.
		//
		// The in-memory replication module just follows the redis entry ID
		// convention so this works fine, but retrieving the creation time
		// would need to be abstracted into a method of the stateReplicator
		// interface if we ever support a different replication layer (for
		// example, pub/sub).
		{
			// Separate logrus instance with its own metadata to aid troubleshooting
			exLogger := logrus.WithFields(logrus.Fields{
				"app":       "open_match",
				"component": "replicatedTicketCache",
				"operation": "expiration",
			})

			// Metrics are tallied in local variables. The values get copied to
			// the module global values that the metrics actually sample after
			// th tally is complete. This way the mid-tally values never get
			// accidentally reported to the metrics sidecar (which could happen
			// if we used the module global values to compute the tally)
			var (
				numInactive            int64
				numInactiveDeletions   int64
				numTickets             int64
				numTicketDeletions     int64
				numAssignments         int64
				numAssignmentDeletions int64
			)
			startTime := time.Now()

			// Cull expired tickets from the local cache inactive ticket set.
			// This expiration is done based on the entry time of the ticket
			// into the system, and the maximum configured ticket TTL rather
			// than when the ticket inactive state was created. This means that
			// it is possible for a ticket that has already expired due to a
			// user-configured expiration time to still persist in the inactive
			// set for a short time. This is fine as an inactive set entry that
			// refers to a non-existent ticket has no effect.
			tc.InactiveSet.Range(func(id, _ any) bool {
				// Get creation timestamp from ID. Timestamp is always assumed
				// to follow the redis stream entry id convention of
				// millisecond precision (13-digit unix timestamp)
				ticketCreationTime, err := strconv.ParseInt(strings.Split(id.(string), "-")[0], 10, 64)
				if err != nil {
					// error; log and go to the next entry
					exLogger.WithFields(logrus.Fields{
						"ticket_id": id,
					}).Error("Unable to parse ticket ID into an unix timestamp when trying to expire old ticket active states")
					return true
				}
				if (time.Now().UnixMilli() - ticketCreationTime) > int64(tc.Cfg.OmCacheTicketTtlMs) {
					// Ensure that when expiring a ticket from the inactive set, the ticket is always deleted as well.
					_, existed := tc.Tickets.LoadAndDelete(id)
					if existed {
						numTickets++
						numTicketDeletions++
					}

					// Remove expired ticket from the inactive set.
					tc.InactiveSet.Delete(id)
					numInactiveDeletions++
				} else {
					numInactive++
				}
				return true
			})

			// cull expired tickets from local cache based on the ticket's expiration time.
			tc.Tickets.Range(func(id, ticket any) bool {
				if time.Now().After(ticket.(*pb.Ticket).GetExpirationTime().AsTime()) {
					tc.Tickets.Delete(id)
					numTicketDeletions++
				} else {
					numTickets++
				}
				return true
			})

			// cull expired assignments from local cache. This uses similar
			// logic to expiring a ticket from the inactive set, but includes
			// an addition configurable TTL so that it is possible for
			// assignments to persist and be retrieved even after the ticket no
			// longer exists in the system.
			//
			// Note that assignment tracking is DEPRECATED functionality
			// intended only for use while doing rapid matchmaker iteration,
			// and we strongly recommend that you use a more robust player
			// status tracking system in production.
			tc.Assignments.Range(func(id, _ any) bool {
				// Get creation timestamp from ID
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
