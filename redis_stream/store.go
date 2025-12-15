package redis_stream

import "regexp"

// An enum for the type of operations that the replication queue can process.
const (
	Ticket = iota
	Activate
	Deactivate
	Assign
)

// StateUpdate チケットの状態に対するあらゆる変更は、StateUpdate としてモデル化されます。
type StateUpdate struct {
	Cmd   int    // The operation this update contains
	Key   string // The key to update
	Value string // The value to associate with this key (if applicable)
}

// StateResponse キャッシュ状態の変更結果。状態レプリケーションは可能な限り更新をバッチ化し、各更新ごとに StateResponse を生成します。
// これは、同時に発生するRPC呼び出しに由来する無関係な更新と一緒にバッチ化される可能性があるため、元の呼び出し元へ返すことができます。
// 主な用途は CreateTicket 呼び出しへの応答を返す仕組みとしての利用で、これにより状態ストレージ層からチケットIDを受け取ります。
// また、内部実装で、ローカルにレプリケートされたチケットキャッシュにどの更新が適用されたかを追跡するためにも使われる場合があります。
// err が nil の場合、result には状態ストレージ実装によってその更新に割り当てられたレプリケーションIDが入ります（例：Redis ではストリームのイベントID）。
// err が nil でない場合、result には失敗した StateUpdate のキーが入り、呼び出し側はどのリクエストでエラーが発生したかを特定するために利用します。
type StateResponse struct {
	Result string
	Err    error
}

// StateReplicator コアの gRPC サーバーは起動時に replicatedTicketCache を生成し、
// このインターフェースに準拠した StateReplicator をインスタンス化することで、om-core の状態をどのようにレプリケートするかを指定します。
type StateReplicator interface {
	GetUpdates() []*StateUpdate
	SendUpdates([]*StateUpdate) []*StateResponse
	GetReplIdValidator() *regexp.Regexp
}
