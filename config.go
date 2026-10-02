package main

import (
	"machine"
	"strconv"
)

// このファイルが「設定の一元化」の場所。
// ピン・キー数・レイヤー数・機種情報は、ここだけ書き換えれば他のファイルに反映される。

// ===== 機種情報(INFOコマンドで返す値の元) =====
const (
	deviceName      = "takoyaki8"
	protocolVersion = 1       // tinykeemap の docs/protocol.md の proto と同じ値
	firmwareVersion = "0.3.0" // ファームウェアのバージョン
)

// ===== ピン割り当て(COL2ROW) =====
// 配列(要素数が決まった並び)で書くと、行数・列数をピンの数から自動で決められる。
var (
	colPins = [...]machine.Pin{machine.GPIO2, machine.GPIO3, machine.GPIO4, machine.GPIO5}
	rowPins = [...]machine.Pin{machine.GPIO0, machine.GPIO1}
)

// ===== マトリクスの大きさ =====
const (
	NumRows   = len(rowPins) // ピンの数から自動で決まる
	NumCols   = len(colPins) // ピンの数から自動で決まる
	NumKeys   = NumRows * NumCols
	NumLayers = 4
)

// 同じ読み取りが何回連続したら状態変化と確定するか
// (1ms間隔でスキャンするので、約5msのチャタリング除去)
const debounceCount = 5

// 予約ピン(今回は未使用。シリーズ共通ルールの記録用)
const (
	pinLED    = machine.GPIO26
	pinSDA    = machine.GPIO14
	pinSCL    = machine.GPIO15
	pinSpare1 = machine.GPIO27
	pinSpare2 = machine.GPIO28
)

// Info は INFO コマンドの応答のうち「OK 」の後ろに続く部分を作る。
// 値はすべて上の設定から作るので、ここを直接書き換える必要はない。
func Info() string {
	return "name=" + deviceName +
		" proto=" + strconv.Itoa(protocolVersion) +
		" fw=" + firmwareVersion +
		" keys=" + strconv.Itoa(NumKeys) +
		" rows=" + strconv.Itoa(NumRows) +
		" cols=" + strconv.Itoa(NumCols) +
		" layers=" + strconv.Itoa(NumLayers)
}
