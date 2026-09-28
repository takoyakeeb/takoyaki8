package main

import "machine"

// マトリクスの大きさ
const (
	NumRows   = 2
	NumCols   = 4
	NumKeys   = NumRows * NumCols
	NumLayers = 1
)

// 同じ読み取りが何回連続したら状態変化と確定するか
// (1ms間隔でスキャンするので、約5msのチャタリング除去)
const debounceCount = 5

// ピン割り当て(COL2ROW)
var (
	colPins = []machine.Pin{machine.GPIO2, machine.GPIO3, machine.GPIO4, machine.GPIO5}
	rowPins = []machine.Pin{machine.GPIO0, machine.GPIO1}
)

// 予約ピン(今回は未使用。シリーズ共通ルールの記録用)
const (
	pinLED    = machine.GPIO26
	pinSDA    = machine.GPIO14
	pinSCL    = machine.GPIO15
	pinSpare1 = machine.GPIO27
	pinSpare2 = machine.GPIO28
)
