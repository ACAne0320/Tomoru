package learn

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"tomoru/internal/scene"
)

type model struct {
	reply         string
	err           error
	system, asked string
}

func (m *model) Chat(_ context.Context, system, user string, _ int) (string, error) {
	m.system, m.asked = system, user
	return m.reply, m.err
}

// The lines and the reply are the experiment's, from qwen3.8-max on the Xiaomi keynote and on the
// Running Man case, with the names it got wrong and right.
var xiaomi = []Line{
	{"在那个时间点舒七的研发呢到了最关键的时候", "At that point, Shu Qi's R&D was at its most critical stage"},
	{"开着亲手打造的车在刘北赛道上驰骋", "to drive a car they built on the Liubei track"},
	{"我们想了挑战刘备", "we wanted to challenge Liu Bei"},
	{"大家咬着牙决定重启 outra项目", "Everyone decided to restart the Outra project."},
	{"对比保时捷", "Compared with Porsche"},
}

const reply = "好的：\n```json\n" + `{"terms":[
 {"heard":"舒七","src":"SU7","tgt":"Xiaomi SU7","sure":true},
 {"heard":"刘北","src":"纽北","tgt":"Nürburgring","sure":true},
 {"heard":"刘备","src":"纽北","tgt":"Nürburgring","sure":true},
 {"heard":"outra","src":"Ultra","tgt":"Ultra","sure":true},
 {"heard":"保时捷","src":"保时捷","tgt":"Porsche","sure":true},
 {"heard":"撸时辰","src":"卢伟冰","tgt":"Lu Weibing","sure":false},
 {"heard":"敖巧香","src":"SU7","tgt":"SU7","sure":true}
]}` + "\n```"

func TestAsk(t *testing.T) {
	m := &model{reply: reply}
	c := Context{SrcLang: "zh", TgtLang: "en", Name: "雷军演讲", About: "小米汽车",
		Terms: []scene.Term{{Src: "雷军", Tgt: "Lei Jun"}}, Rejected: []string{"大厄災"}}
	got, err := Ask(context.Background(), m, c, xiaomi)
	if err != nil {
		t.Fatal(err)
	}
	want := []Suggestion{
		{Src: "SU7", Tgt: "Xiaomi SU7", Heard: []string{"舒七"}, Count: 1, Example: xiaomi[0].Src},
		{Src: "纽北", Tgt: "Nürburgring", Heard: []string{"刘北", "刘备"}, Count: 2, Example: xiaomi[1].Src},
		{Src: "Ultra", Tgt: "Ultra", Heard: []string{"outra"}, Count: 1, Example: xiaomi[3].Src},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
	for _, s := range []string{"中文原文", "英语译文"} {
		if !strings.Contains(m.system, s) {
			t.Errorf("system prompt %q", m.system)
		}
	}
	for _, s := range []string{"在看：雷军演讲\n小米汽车", "雷军 → Lei Jun", "不要再写：大厄災", "原文：对比保时捷\n译文：Compared with Porsche"} {
		if !strings.Contains(m.asked, s) {
			t.Errorf("no %q in %q", s, m.asked)
		}
	}

	if _, err := Ask(context.Background(), &model{reply: "Sorry, I can't."}, c, xiaomi); err == nil {
		t.Error("no error without JSON")
	}
	quota := errors.New("quota")
	if _, err := Ask(context.Background(), &model{err: quota}, c, xiaomi); !errors.Is(err, quota) {
		t.Errorf("err %v", err)
	}
}

// TestFilter keeps what the experiment found worth suggesting.
func TestFilter(t *testing.T) {
	lines := []Line{
		{"개미들 모인다 지금 개미들 모여요", "蚂蚁们聚过来了"},
		{"엔터주로 내가 300원짜리 살게", "我买300韩元的"},
		{"Private Cloud Compute extends that", "私有云计算也能提供同样级别的隐私保护"},
		{"インフィアとお揃いのリボン", "和英菲亚同款的蝴蝶结"},
		{"メデオの脅威", "梅朵的威胁"},
		{"F, the season", "F，这一季"},
	}
	c := Context{Terms: []scene.Term{
		{Src: "メテオ", Tgt: "陨石", Heard: []string{"メデオ"}, Auto: true},
		{Src: "ニンフィア", Tgt: "仙子伊布"}, // the user's
	}, Rejected: []string{"개미"}}
	fs := []found{
		{Heard: "개미들", Src: "개미", Tgt: "散户", Sure: true},                                         // a particle: as 개미, which was turned down
		{Heard: "엔터주로", Src: "엔터주", Tgt: "娱乐股", Sure: true},                                      // a particle: a translation only
		{Heard: "private cloud compute", Src: "Private Cloud Compute", Tgt: "私有云计算", Sure: true}, // case apart, and translated right
		{Heard: "インフィア", Src: "ニンフィア", Tgt: "仙女伊布", Sure: true},                                  // the user's own term
		{Heard: "メデオ", Src: "メテオ", Tgt: "陨石", Sure: true},                                        // the glossary has it
		{Heard: "シュート", Src: "キュート", Tgt: "可爱", Sure: true},                                      // not in the lines
	}
	got := Filter(fs, c, lines)
	want := []Suggestion{{Src: "엔터주", Tgt: "娱乐股", Count: 1, Example: lines[1].Src}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestMerge(t *testing.T) {
	ss := Merge(nil, Suggestion{Src: "纽北", Tgt: "Nürburgring", Heard: []string{"刘北"}, Count: 2, Example: "a"})
	ss = Merge(ss, Suggestion{Src: "纽北", Tgt: "纽博格林", Heard: []string{"刘备", "刘北"}, Count: 1, Example: "b"})
	ss = Merge(ss, Suggestion{Src: "SU7", Tgt: "SU7", Count: 1})
	want := []Suggestion{{Src: "纽北", Tgt: "Nürburgring", Heard: []string{"刘北", "刘备"}, Count: 3, Example: "a"}, {Src: "SU7", Tgt: "SU7", Count: 1}}
	if !reflect.DeepEqual(ss, want) {
		t.Errorf("got  %+v\nwant %+v", ss, want)
	}
}
