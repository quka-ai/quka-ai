package utils

import (
	"github.com/abadojack/whatlanggo"
)

var whatLangOpts = whatlanggo.Options{
	Whitelist: map[whatlanggo.Lang]bool{
		whatlanggo.Eng: true,
		whatlanggo.Rus: true,
		whatlanggo.Cmn: true,
		whatlanggo.Fra: true,
		// ... pls issus
	},
}

func WhatLang(query string) string {
	info := whatlanggo.DetectWithOptions(query, whatLangOpts)
	// fmt.Println("Language:", info.Lang.String(), " Script:", whatlanggo.Scripts[info.Script], " Confidence: ", info.Confidence)
	return info.Lang.String()
}
