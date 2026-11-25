package main

// 部品(LED、OLED など)の決まり。
// 機種のファイル(model_*.go)が、使う部品を parts に並べてコアに渡す。
// コア(main.go)は、部品の中身を知らずに、決まった時に呼ぶだけ。ない部品は、並べなければよい。
// 前例は scanner.go の Scanner(入力元の決まり)。
type Part interface {
	Start()      // 起動時に1回だけ呼ばれる
	Tick()       // メインループ(約1ms)ごとに呼ばれる。更新の間隔は、部品が自分で数える
	KeyPressed() // どれかのキーを押したときに呼ばれる

	// CfgDefs は、この部品の設定の項目(名前・初期値・種類)を defs の後ろに足して返す。
	// 設定の初期値は、部品が持つ。並べた順が、CFG LIST と保存のときの並びになる(cfg.go の buildCfgDefs)。
	// Start より前に呼ばれることがあるので、機種のファイルの値(ピンなど)には頼らない。
	CfgDefs(defs []cfgDef) []cfgDef

	// CfgOptions は、設定 name の候補の一覧を返す(CFG LIST で返す)。この部品の名前でなければ nil。
	CfgOptions(name string) []string
}
