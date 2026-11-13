package main

import (
	"machine"
	"strconv"
)

// このファイルは、機種によらない設定(ファームのバージョン、更新の間隔など)と、INFO の応答を作る場所。
// 機種ごとの値(機種名、ピン、レイヤー数、LED・OLED、初期キーマップ)は、model_takoyaki8.go にある。

// ===== ファームの情報(INFOコマンドで返す値の元) =====
const (
	protocolVersion = 1       // tinykeemap の docs/protocol.md の proto と同じ値
	firmwareVersion = "0.5.0" // ファームウェアのバージョン
)

// ===== マトリクスの大きさ =====
// 行数・列数は、機種のファイルのピンの数から自動で決まる。
const (
	NumRows = len(rowPins) // ピンの数から自動で決まる
	NumCols = len(colPins) // ピンの数から自動で決まる
	NumKeys = NumRows * NumCols
)

// 同じ読み取りが何回連続したら状態変化と確定するか
// (1ms間隔でスキャンするので、約5msのチャタリング除去)
const debounceCount = 5

// ===== LED(WS2812B)の設定 =====
// ピン・個数・明るさの上限は、model_takoyaki8.go。
const (
	// 通常の LED の表示を更新する間隔。メインループ(約1ms)が何回まわるたびに1回更新するか。
	// 100 なら約0.1秒ごと。1 にすると毎回更新する(いちばん厳しい確認用)。
	// 動いている効果の更新は、下の ledFastLoops。
	ledEveryLoops = 100

	// 動いている効果(press で光っているあいだ、breathe)の更新の間隔。
	// 20 なら約20msごと(明るさを、なめらかに変えるため)。
	ledFastLoops = 20

	// press の効果(キーを押すと光って、消えていく)が、消えるまでの時間。
	ledPressFadeMs = 600 // ミリ秒。300 だと余韻が短かったので、600 にした

	// breathe の効果(ゆっくり明るさが変わる)の、1周の時間(ミリ秒)と、いちばん暗いときに残す明るさ(%。led の明るさに対して)。
	ledBreathePeriodMs = 4000
	ledBreatheMinPct   = 10
)

// ===== OLED(SSD1306、I2C)の設定 =====
// ピン・アドレス・画面の大きさは、model_takoyaki8.go。
const (
	// 通信の速さ。乱れるときは 100 * machine.KHz に下げる。
	oledFreq = 400 * machine.KHz

	// OLED の表示を更新する間隔。メインループ(約1ms)が何回まわるたびに1回更新するか。
	// 100 なら約100msごと。1 にすると毎回更新する(いちばん厳しい確認用)。
	oledEveryLoops = 100
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
		" layers=" + strconv.Itoa(NumLayers) +
		" cfg=1" + // 設定(CFG コマンド)に対応している
		" lname=" + strconv.Itoa(cfgLnameMax) + // レイヤー名の最大の文字数(cfg.go)
		" saved=" + savedFlag() // 電源を入れ直しても同じ状態なら 1(state.go)
}
