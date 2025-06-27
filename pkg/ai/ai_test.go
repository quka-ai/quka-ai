package ai

import "testing"

func Test_TimeTpl(t *testing.T) {
	tpl := GenerateTimeListAtNowCN()

	t.Log(tpl)
}
