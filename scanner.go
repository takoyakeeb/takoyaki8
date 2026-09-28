package main

// KeyEvent は「キー番号IDが押された/離された」という出来事を表す。
// 入力元(マトリクス、エンコーダ、分割の相手側)が何であっても、この形にそろえる。
type KeyEvent struct {
	ID      uint16
	Pressed bool
}

// Scanner は入力元の共通の形。
// 新しく見つかったイベントを buf の後ろに追加して返す。
type Scanner interface {
	Scan(buf []KeyEvent) []KeyEvent
}
