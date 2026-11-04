package main

// LED(WS2812B)の制御。データ線は config.go の pinLED(GPIO26)。
//
// 設定(cfg.go)で動きを決める。
//   led_mode: 効果の名前。
//     layer … 4個とも、いまのレイヤー(currentLayer)の色で点灯する(初期値)。
//     press … 何もしていないときは消灯。キーを押すと、4個とも、いまのレイヤーの色で光って、少しして消える。
//     off   … 消灯。
//     test  … 白い点が、端から端まで動き続ける(配線の診断用。tinykeemap の画面には出さない)。
//     ファームが知らない名前は、layer と同じ動きにする。
//   lcolor0〜: レイヤーごとの色(6桁の16進)。
//   led: 明るさ(0〜100%)。100% は ledMaxLevel(config.go)。見た目が比例するよう、ledLevelFor で補正する。
// 設定を変えると、次の更新(約 ledEveryLoops ms 後)で、LED に出る。

import (
	"image/color" // 色(赤・緑・青の値)を表す標準の型
	"machine"
	"strconv"
	"time" // press の、光ってからの時間の計測に使う

	"tinygo.org/x/drivers/ws2812" // WS2812 にデータを送るドライバ
)

var (
	ledDev    ws2812.Device       // ドライバ(これを通して LED に色を送る)
	ledColors [NumLEDs]color.RGBA // NumLEDs 個ぶんの色。送る前にここへ書き込む
	ledPos    int                 // test のとき、白い点がいる LED の番号
	ledTicks  int                 // メインループが何回まわったか(ledWait に達したら更新して0に戻す)
	ledWait   = ledEveryLoops    // 次の更新までに、メインループが何回まわるか(press で光っているあいだだけ、短くなる)

	ledPressed bool      // press: キーを押してから、まだ消えていなければ true
	ledPressAt time.Time // press: 最後にキーを押した時刻
)

// initLED は、データ線を出力にして、ドライバを作り、全部の LED を消す。
func initLED() {
	pin := pinLED
	pin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	ledDev = ws2812.NewWS2812(pin)

	// ledColors は最初、全部 0(消灯)なので、そのまま送ると全部消える
	ledDev.WriteColors(ledColors[:])
}

// ledLevelFor は、明るさの設定(0〜100%)を、LED に送る明るさの上限(0〜255)に直す。
// 人の目は、暗い所の変化に敏感なので、比例ではなく2乗にする(50% で、見た目がおよそ半分)。
// 0% は 0(消灯)。1% 以上は、最低でも 1 にする。
func ledLevelFor(percent int) uint8 {
	v := ledMaxLevel * percent * percent / 10000
	if percent > 0 && v == 0 {
		v = 1
	}
	return uint8(v)
}

// hexNibble は、16進の1文字(0-9、A-F。cfgColor が大文字にそろえている)を、0〜15 にする。
func hexNibble(c byte) uint8 {
	if c >= 'A' {
		return c - 'A' + 10
	}
	return c - '0'
}

// parseColor は、6桁の16進の色(例 "2060FF")を、赤・緑・青にする。6桁でなければ白。
func parseColor(s string) color.RGBA {
	if len(s) != 6 {
		return color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}
	return color.RGBA{
		R: hexNibble(s[0])<<4 | hexNibble(s[1]),
		G: hexNibble(s[2])<<4 | hexNibble(s[3]),
		B: hexNibble(s[4])<<4 | hexNibble(s[5]),
		A: 255,
	}
}

// ledScale は、色 c の明るさを、level/255 倍にする(level = 255 なら、そのまま)。
func ledScale(c color.RGBA, level uint8) color.RGBA {
	return color.RGBA{
		R: uint8(uint16(c.R) * uint16(level) / 255),
		G: uint8(uint16(c.G) * uint16(level) / 255),
		B: uint8(uint16(c.B) * uint16(level) / 255),
		A: 255,
	}
}

// ledLayerColor は、レイヤー l の色(設定 lcolor<l>)を返す。
func ledLayerColor(l int) color.RGBA {
	v, _ := cfgGet("lcolor" + strconv.Itoa(l))
	return parseColor(v)
}

// ledKeyPressed は、キーを押したときに、メインループから呼ばれる(どのキーでも)。
// press の効果の合図で、次の tickLED で、すぐ光らせる。
func ledKeyPressed() {
	ledPressed = true
	ledPressAt = time.Now()
	ledTicks = ledWait // 次の tickLED で、待たずに更新する
}

// ledPressFade は、press の光の強さ(0〜255)を返す。押した直後は 255 で、ledPressFadeMs かけて 0 になる。
// 0 になったら、ledPressed を false にする。
func ledPressFade() int {
	if !ledPressed {
		return 0
	}
	dur := time.Duration(ledPressFadeMs) * time.Millisecond
	el := time.Since(ledPressAt)
	if el >= dur {
		ledPressed = false
		return 0
	}
	return int(255 - int64(el)*255/int64(dur))
}

// tickLED は、メインループから毎回呼ばれる。
// ledWait 回に1回だけ、設定にしたがって色を決めて、LED に送る。
func tickLED() {
	ledTicks++
	if ledTicks < ledWait {
		return
	}
	ledTicks = 0
	ledWait = ledEveryLoops // press で光っているときだけ、下で短くする

	level := ledLevelFor(cfgInt("led"))
	mode, _ := cfgGet("led_mode")

	// いったん全部消してから、効果ごとに色を決める
	for i := range ledColors {
		ledColors[i] = color.RGBA{}
	}
	switch mode {
	case "off":
		// 消灯のまま
	case "press":
		// 押してから消えるまで、レイヤーの色で、だんだん暗くする。何もないときは、消灯のまま
		if fade := ledPressFade(); fade > 0 {
			ledWait = ledPressFastLoops
			c := ledScale(ledLayerColor(currentLayer()), uint8(int(level)*fade/255))
			for i := range ledColors {
				ledColors[i] = c
			}
		}
	case "test":
		ledPos = (ledPos + 1) % NumLEDs
		ledColors[ledPos] = ledScale(color.RGBA{R: 255, G: 255, B: 255, A: 255}, level)
	default:
		// layer と、ファームが知らない効果の名前
		c := ledScale(ledLayerColor(currentLayer()), level)
		for i := range ledColors {
			ledColors[i] = c
		}
	}

	// NumLEDs 個ぶんまとめて LED に送る。この間(1個あたり約30μs。4個なら約0.12ms)は割り込みが止まる場合がある
	ledDev.WriteColors(ledColors[:])
}
