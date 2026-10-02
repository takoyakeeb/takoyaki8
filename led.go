package main

// LED(WS2812B)の制御。データ線は config.go の pinLED(GPIO26)。
//
// 今は「配線とコードの確認」用で、tinykeemap からは設定できない(コマンドは作っていない)。
// 確認用の動き: 白い点が、端から端まで動き続ける。
// 点を動かしている間も、キー入力・キーの読み取り・USBシリアルが、これまでどおり動くかを確かめる。

import (
	"image/color" // 色(赤・緑・青の値)を表す標準の型
	"machine"

	"tinygo.org/x/drivers/ws2812" // WS2812 にデータを送るドライバ
)

var (
	ledDev    ws2812.Device        // ドライバ(これを通して LED に色を送る)
	ledColors [NumLEDs]color.RGBA // 10個ぶんの色。送る前にここへ書き込む
	ledPos    int                  // 白い点がいる LED の番号
	ledTicks  int                  // メインループが何回まわったか(ledEveryLoops に達したら更新して0に戻す)
)

// initLED は、データ線を出力にして、ドライバを作り、全部の LED を消す。
func initLED() {
	pin := pinLED
	pin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	ledDev = ws2812.NewWS2812(pin)

	// ledColors は最初、全部 0(消灯)なので、そのまま送ると全部消える
	ledDev.WriteColors(ledColors[:])
}

// tickLED は、メインループから毎回呼ばれる。
// ledEveryLoops 回に1回だけ、白い点を1つ進めて、LED に送る。
func tickLED() {
	ledTicks++
	if ledTicks < ledEveryLoops {
		return
	}
	ledTicks = 0

	// いまの点を消して、次の LED に点を移す(端まで行ったら最初に戻る)
	ledColors[ledPos] = color.RGBA{}
	ledPos = (ledPos + 1) % NumLEDs
	ledColors[ledPos] = color.RGBA{R: ledLevel, G: ledLevel, B: ledLevel, A: 255}

	// 10個ぶんまとめて LED に送る。この間(約0.3ms)は割り込みが止まる場合がある
	ledDev.WriteColors(ledColors[:])
}
