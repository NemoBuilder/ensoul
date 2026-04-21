package methodology

import "testing"

func TestDetectScenario(t *testing.T) {
	cases := []struct {
		msg  string
		want Scenario
	}{
		{"帮我写一条推文，关于AI的", ScenarioWriting},
		{"draft a tweet about my new product", ScenarioWriting},
		{"我没灵感，今天发什么好", ScenarioTopic},
		{"give me some tweet ideas", ScenarioTopic},
		{"这条推文怎么样：xxx", ScenarioReview},
		{"帮我看看这条thread", ScenarioReview},
		{"怎么涨粉？", ScenarioGrowth},
		{"how to grow my X account", ScenarioGrowth},
		{"帮我诊断一下我的账号", ScenarioDiagnosis},
		{"帮我更新一下我的定位", ScenarioMemory},
		{"update my profile", ScenarioMemory},
		{"remember this for later", ScenarioMemory},
		{"你好", ScenarioGeneral},
		{"", ScenarioGeneral},
	}
	for _, c := range cases {
		got := DetectScenario(c.msg)
		if got != c.want {
			t.Errorf("DetectScenario(%q) = %s, want %s", c.msg, got, c.want)
		}
	}
}
