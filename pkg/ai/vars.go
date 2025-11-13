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

var CurrentSymbols = strings.Join([]string{"$hidden[...]"}, ",")

var (
	SITE_TITLE       = "Quka"
	SITE_DESCRIPTION = "QukaAI，快速构建个人第二大脑"
)

func RegisterConstants(siteTitle, siteDescription string) {
	SITE_TITLE = siteTitle
	SITE_DESCRIPTION = siteDescription
}
