// Package samples is a made-up stream, as a streamer would say it, in the languages most watched:
// what the subtitles show to show their look, and what the first-run guide says aloud to try
// translating.
package samples

// Lines are the sentences, in the order they are said, by language: Chinese, in both scripts,
// English, Japanese and Korean.
var Lines = []map[string]string{
	{"zh": "大家晚上好。", "ja": "皆さん、こんばんは。", "en": "Good evening, everyone.", "ko": "여러분, 안녕하세요.", "zh-Hant": "大家晚上好。"},
	{"zh": "今天的直播会比平时长一点。", "ja": "今日の配信はいつもより少し長くなります。", "en": "Today's stream will run a bit longer than usual.", "ko": "오늘 방송은 평소보다 조금 길어질 거예요.", "zh-Hant": "今天的直播會比平常長一點。"},
	{"zh": "先看一下评论吧。", "ja": "まずはコメントを見てみましょう。", "en": "Let's check the comments first.", "ko": "먼저 댓글부터 볼게요.", "zh-Hant": "先看一下留言吧。"},
	{"zh": "谢谢大家的礼物！", "ja": "みんな、ギフトありがとう！", "en": "Thanks for the gifts, everyone!", "ko": "선물 보내주셔서 감사해요!", "zh-Hant": "謝謝大家的禮物！"},
	{"zh": "好，那我们开始吧。", "ja": "よし、じゃあ始めましょう。", "en": "Okay, let's get started.", "ko": "자, 그럼 시작해 볼게요.", "zh-Hant": "好，那我們開始吧。"},
	{"zh": "上次卡在了第三章的最后。", "ja": "前回は第三章の最後で詰まっちゃいました。", "en": "Last time I got stuck at the end of chapter three.", "ko": "지난번엔 3장 마지막에서 막혔어요.", "zh-Hant": "上次卡在了第三章的最後。"},
	{"zh": "这次准备了更好的装备。", "ja": "今回はもっといい装備を用意してきました。", "en": "This time I've got better gear.", "ko": "이번엔 더 좋은 장비를 준비했어요.", "zh-Hant": "這次準備了更好的裝備。"},
	{"zh": "今天我想把这个新游戏通关。", "ja": "今日はこの新しいゲームを最後までクリアしたいと思います。", "en": "Today I want to beat this new game.", "ko": "오늘은 이 새 게임을 끝까지 깨고 싶어요.", "zh-Hant": "今天我想把這款新遊戲破關。"},
	{"zh": "啊，Boss 出来了！", "ja": "あ、ボスが出てきた！", "en": "Oh, here comes the boss!", "ko": "아, 보스 나왔다!", "zh-Hant": "啊，Boss 出來了！"},
	{"zh": "这个 Boss 的第二阶段超强的。", "ja": "このボスの第二形態がめっちゃ強いんだよね。", "en": "This boss's second phase is really tough.", "ko": "이 보스 2페이즈가 진짜 강해요.", "zh-Hant": "這個 Boss 的第二階段超強的。"},
}

// Has tells whether there are samples in lang.
func Has(lang string) bool {
	_, ok := Lines[0][lang]
	return ok
}

// In is the sample s in lang, or where there is none, in the language nearest it that has one:
// Traditional Chinese for Cantonese, which is written so, and English for the others.
func In(s map[string]string, lang string) string {
	if t, ok := s[lang]; ok {
		return t
	}
	if lang == "yue" {
		return s["zh-Hant"]
	}
	return s["en"]
}

// Spoken are the sentences the guide says aloud, in lang, of those Has: the first few.
func Spoken(lang string) []string {
	var out []string
	for _, l := range Lines[:4] {
		out = append(out, l[lang])
	}
	return out
}

// Dialogue is a game's dialogue box, by language: who speaks and what they say. The guide has it
// translated as a picture (assets.SamplePicture).
var Dialogue = map[string][2]string{
	"zh": {"艾露娜", "克劳德，塔顶上那个发光的东西……该不会就是「星之碎片」吧？"},
	"ja": {"エルナ", "クロード、あの塔のてっぺんで光ってるもの……まさか「星の欠片」じゃないよね？"},
	"en": {"Erna", "Claude, that light on top of the tower… it can't be the Star Fragment, can it?"},
	"ko": {"에르나", "클로드, 저 탑 꼭대기에서 빛나는 거… 설마 ‘별의 조각’은 아니겠지?"},
}
