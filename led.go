package main

// LED(WS2812B)の部品。データ線のピン・個数・明るさの上限は、機種のファイル(model_*.go)が
// ledPart に入れて渡す(takoyaki8 は GPIO26、4個)。
// 1つの機種で使える LED の部品は1つだけ(中で使う変数を、部品ごとに分けていないため)。
//
// 設定(cfg.go)で動きを決める。
//   led_mode: 効果の名前。
//     layer … 4個とも、いまのレイヤー(currentLayer)の色で点灯する(初期値)。
//     breathe … 4個とも、いまのレイヤーの色で、ゆっくり明るくなったり暗くなったりする。
//     press … 何もしていないときは消灯。キーを押すと、4個とも、いまのレイヤーの色で光って、少しして消える。
//     off   … 消灯。
//     test  … 白い点が、端から端まで動き続ける(配線の診断用。tinykeemap の画面には出さない)。
//     ファームが知らない名前は、layer と同じ動きにする。
//   lcolor0〜: レイヤーごとの色(6桁の16進)。
//   led: 明るさ(0〜100%)。100% は ledPart の maxLevel。見た目が比例するよう、ledLevelFor で補正する。
// 設定を変えると、次の更新(約 ledEveryLoops ms 後)で、LED に出る。

import (
	"image/color" // 色(赤・緑・青の値)を表す標準の型
	"machine"
	"strconv"
	"time" // press の、光ってからの時間の計測に使う

	"tinygo.org/x/drivers/ws2812" // WS2812 にデータを送るドライバ
)

// ledPart は、LED の部品。機種のファイルが値を入れて、parts に並べる。
type ledPart struct {
	pin      machine.Pin // データ線のピン
	count    int         // つないだ LED の数
	maxLevel int         // 設定 led が 100% のときの、赤・緑・青それぞれの値(0〜255)
}

func (p *ledPart) Start() {
	ledConf = p
	initLED()
}
func (p *ledPart) Tick()       { tickLED() }
func (p *ledPart) KeyPressed() { ledKeyPressed() }

// レイヤーごとの色の初期値(段階8bで、このままに決定)
var lcolorDefaults = [NumLayers]string{"2060FF", "FF8000", "20FF40", "FF2080"}

// ledModeOptions は、led_mode の候補(画面に出す選択肢)。CFG LIST で返す。
// test(診断用)は、画面に出さないので、入れない。
// CFG SET led_mode には、候補にない名前も入れられる(知らない名前は layer と同じ動き)。
var ledModeOptions = []string{"layer", "press", "breathe", "off"}

// CfgDefs は、LED の設定の項目を足す(led、led_mode、lcolor0〜)。
func (p *ledPart) CfgDefs(defs []cfgDef) []cfgDef {
	defs = append(defs,
		cfgDef{"led", "80", cfgPercent}, // 3Dプリントのケース(バンパー)を付けた状態で、ちょうどよい明るさ(段階8bで決定。LED に送る値は 163)
		cfgDef{"led_mode", "layer", cfgWord},
	)
	for l := 0; l < NumLayers; l++ {
		defs = append(defs, cfgDef{"lcolor" + strconv.Itoa(l), lcolorDefaults[l], cfgColor})
	}
	return defs
}

// CfgOptions は、led_mode の候補を返す。
func (p *ledPart) CfgOptions(name string) []string {
	if name == "led_mode" {
		return ledModeOptions
	}
	return nil
}

var (
	ledConf   *ledPart        // 使っている LED の部品(Start で入る)
	ledDev    ws2812.Device   // ドライバ(これを通して LED に色を送る)
	ledColors []color.RGBA    // LED の数ぶんの色。送る前にここへ書き込む(initLED で作る)
	ledPos    int             // test のとき、白い点がいる LED の番号
	ledTicks  int             // メインループが何回まわったか(ledWait に達したら更新して0に戻す)
	ledWait   = ledEveryLoops // 次の更新までに、メインループが何回まわるか(press で光っているあいだだけ、短くなる)

	ledEpoch time.Time // breathe: 起動した時刻(ここからの時間で、明るさを決める)

	ledPressed bool      // press: キーを押してから、まだ消えていなければ true
	ledPressAt time.Time // press: 最後にキーを押した時刻
)

// initLED は、データ線を出力にして、ドライバを作り、全部の LED を消す。
func initLED() {
	ledColors = make([]color.RGBA, ledConf.count)
	pin := ledConf.pin
	pin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	ledDev = ws2812.NewWS2812(pin)
	ledEpoch = time.Now()

	// ledColors は最初、全部 0(消灯)なので、そのまま送ると全部消える
	ledDev.WriteColors(ledColors[:])
}

// ledLevelFor は、明るさの設定(0〜100%)を、LED に送る明るさの上限(0〜255)に直す。
// 人の目は、暗い所の変化に敏感なので、比例ではなく2乗にする(50% で、見た目がおよそ半分)。
// 0% は 0(消灯)。1% 以上は、最低でも 1 にする。
func ledLevelFor(percent int) uint8 {
	v := ledConf.maxLevel * percent * percent / 10000
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

// ledBreatheFactor は、breathe の明るさ(0〜255)を返す。ledBreathePeriodMs で1周する。
// 三角波(上がって下がる)を、smoothstep(なめらかな S 字)で丸めて、いちばん暗いときも ledBreatheMinPct % を残す。
func ledBreatheFactor() int {
	period := int64(ledBreathePeriodMs)
	phase := int64(time.Since(ledEpoch)/time.Millisecond) % period
	half := period / 2
	var t int64 // 三角波(0〜255)
	if phase < half {
		t = phase * 255 / half
	} else {
		t = (period - phase) * 255 / half
	}
	s := t * t * (3*255 - 2*t) / (255 * 255) // smoothstep。0〜255
	floor := int64(ledBreatheMinPct) * 255 / 100
	return int(floor + (255-floor)*s/255)
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
	case "breathe":
		ledWait = ledFastLoops
		c := ledScale(ledLayerColor(currentLayer()), uint8(int(level)*ledBreatheFactor()/255))
		for i := range ledColors {
			ledColors[i] = c
		}
	case "press":
		// 押してから消えるまで、レイヤーの色で、だんだん暗くする。何もないときは、消灯のまま
		if fade := ledPressFade(); fade > 0 {
			ledWait = ledFastLoops
			c := ledScale(ledLayerColor(currentLayer()), uint8(int(level)*fade/255))
			for i := range ledColors {
				ledColors[i] = c
			}
		}
	case "test":
		ledPos = (ledPos + 1) % len(ledColors)
		ledColors[ledPos] = ledScale(color.RGBA{R: 255, G: 255, B: 255, A: 255}, level)
	default:
		// layer と、ファームが知らない効果の名前
		c := ledScale(ledLayerColor(currentLayer()), level)
		for i := range ledColors {
			ledColors[i] = c
		}
	}

	// LED の数ぶんまとめて LED に送る。この間(1個あたり約30μs。4個なら約0.12ms)は割り込みが止まる場合がある
	ledDev.WriteColors(ledColors[:])
}
