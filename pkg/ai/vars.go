package ai

import "strings"

const (
	PROMPT_VAR_TIME_RANGE       = "${time_range}"
	PROMPT_VAR_LANG             = "${lang}"
	PROMPT_VAR_HISTORIES        = "${histories}"
	PROMPT_VAR_SYMBOL           = "${symbol}"
	PROMPT_VAR_RELEVANT_PASSAGE = "${relevant_passage}"
	PROMPT_VAR_SITE_TITLE       = "${site_title}"
	PROMPT_VAR_QUERY            = "${query}"
)

var CurrentSymbols = strings.Join([]string{"$hidden[xxxx]"}, ",")

var (
	SITE_TITLE       = "Quka"
	SITE_DESCRIPTION = "QukaAI，快速构建个人第二大脑"
)

func RegisterConstants(siteTitle, siteDescription string) {
	SITE_TITLE = siteTitle
	SITE_DESCRIPTION = siteDescription
}

const APPEND_PROMPT_CN = `
系统支持的 Markdown 数学公式语法需要使用 ${math}$ 包住表示inline，否则使用
$$
{math}
$$
包住表示block。
系统内置了脱敏语法"$hidden[xxxx]"，当你发现参考内容中出现了这些语法信息，请不要做任何处理，直接原封不动的响应出来，前端会进行处理。
`
