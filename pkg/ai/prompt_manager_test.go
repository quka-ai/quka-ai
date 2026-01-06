package ai

import (
	"strings"
	"testing"

	"github.com/quka-ai/quka-ai/pkg/types"
)

func TestPromptTemplate_Build(t *testing.T) {
	tests := []struct {
		name     string
		template *PromptTemplate
		want     string
	}{
		{
			name: "basic template without vars",
			template: &PromptTemplate{
				Header: "Header",
				Body:   "Body",
				Append: "Append",
				Lang:   MODEL_BASE_LANGUAGE_CN,
				Vars:   make(map[string]string),
			},
			want: "Header\n\nBody\n\nAppend",
		},
		{
			name: "template with variables",
			template: &PromptTemplate{
				Header: "Header ${var1}",
				Body:   "Body ${var2}",
				Append: "Append ${var3}",
				Lang:   MODEL_BASE_LANGUAGE_CN,
				Vars: map[string]string{
					"${var1}": "value1",
					"${var2}": "value2",
					"${var3}": "value3",
				},
			},
			want: "Header value1\n\nBody value2\n\nAppend value3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.template.Build()
			if got != tt.want {
				t.Errorf("Build() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPromptTemplate_SetVar(t *testing.T) {
	template := &PromptTemplate{
		Header: "Header ${var1}",
		Body:   "Body ${var2}",
		Append: "Append",
		Lang:   MODEL_BASE_LANGUAGE_CN,
		Vars:   make(map[string]string),
	}

	template.SetVar("var1", "value1")
	template.SetVar("var2", "value2")

	built := template.Build()
	want := "Header value1\n\nBody value2\n\nAppend"
	if built != want {
		t.Errorf("Build() = %v, want %v", built, want)
	}
}

func TestPromptTemplate_SetBody(t *testing.T) {
	template := &PromptTemplate{
		Header: "Header",
		Body:   "",
		Append: "Append",
		Lang:   MODEL_BASE_LANGUAGE_CN,
		Vars:   make(map[string]string),
	}

	template.SetBody("New Body")

	built := template.Build()
	want := "Header\n\nNew Body\n\nAppend"
	if built != want {
		t.Errorf("Build() = %v, want %v", built, want)
	}
}

func TestPromptManager_NewPromptManager(t *testing.T) {
	config := &PromptConfig{
		Header:       "Custom Header",
		ChatSummary:  "Custom Summary",
		EnhanceQuery: "Custom Enhance",
		SessionName:  "Custom Session",
	}

	pm := NewPromptManager(config, MODEL_BASE_LANGUAGE_CN)

	if pm == nil {
		t.Fatal("NewPromptManager() returned nil")
	}

	if pm.config != config {
		t.Error("NewPromptManager() config not set correctly")
	}

	if pm.lang != MODEL_BASE_LANGUAGE_CN {
		t.Error("NewPromptManager() lang not set correctly")
	}

	// Verify default prompts are initialized
	if pm.defaultPrompts["chat"] == nil {
		t.Error("NewPromptManager() chat default prompt not initialized")
	}
	if pm.defaultPrompts["rag"] == nil {
		t.Error("NewPromptManager() rag default prompt not initialized")
	}
	if pm.defaultPrompts["summary"] == nil {
		t.Error("NewPromptManager() summary default prompt not initialized")
	}
	if pm.defaultPrompts["enhance_query"] == nil {
		t.Error("NewPromptManager() enhance_query default prompt not initialized")
	}
	if pm.defaultPrompts["butler"] == nil {
		t.Error("NewPromptManager() butler default prompt not initialized")
	}
	if pm.defaultPrompts["journal"] == nil {
		t.Error("NewPromptManager() journal default prompt not initialized")
	}
}

func TestPromptManager_GetChatTemplate(t *testing.T) {
	tests := []struct {
		name   string
		config *PromptConfig
		space  *types.Space
		lang   string
	}{
		{
			name:   "CN template without space",
			config: nil,
			space:  nil,
			lang:   MODEL_BASE_LANGUAGE_CN,
		},
		{
			name:   "EN template without space",
			config: nil,
			space:  nil,
			lang:   MODEL_BASE_LANGUAGE_EN,
		},
		{
			name:   "CN template with space custom prompts",
			config: nil,
			space: &types.Space{
				BasePrompt: "Custom base prompt",
				ChatPrompt: "Custom chat prompt",
			},
			lang: MODEL_BASE_LANGUAGE_CN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewPromptManager(tt.config, MODEL_BASE_LANGUAGE_CN)
			template := pm.GetChatTemplate(tt.lang, tt.space)

			if template == nil {
				t.Fatal("GetChatTemplate() returned nil")
			}

			// Verify structure
			if template.Header == "" {
				t.Error("GetChatTemplate() header is empty")
			}
			if template.Body == "" {
				t.Error("GetChatTemplate() body is empty")
			}
			if template.Append == "" {
				t.Error("GetChatTemplate() append is empty")
			}

			// Verify space custom prompts are included
			if tt.space != nil && tt.space.BasePrompt != "" {
				if !strings.Contains(template.Body, tt.space.BasePrompt) {
					t.Error("GetChatTemplate() does not include space BasePrompt")
				}
			}
			if tt.space != nil && tt.space.ChatPrompt != "" {
				if !strings.Contains(template.Body, tt.space.ChatPrompt) {
					t.Error("GetChatTemplate() does not include space ChatPrompt")
				}
			}

			// Verify BASE_GENERATE_PROMPT is always included
			built := template.Build()
			if !strings.Contains(built, "Tool Usage Guidelines") && !strings.Contains(built, "工具使用指导原则") {
				t.Error("GetChatTemplate() does not include base generate prompt")
			}
		})
	}
}

func TestPromptManager_GetRAGTemplate(t *testing.T) {
	tests := []struct {
		name  string
		space *types.Space
		lang  string
	}{
		{
			name:  "CN template",
			space: nil,
			lang:  MODEL_BASE_LANGUAGE_CN,
		},
		{
			name:  "EN template",
			space: nil,
			lang:  MODEL_BASE_LANGUAGE_EN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewPromptManager(nil, MODEL_BASE_LANGUAGE_CN)
			template := pm.GetRAGTemplate(tt.lang, tt.space)

			if template == nil {
				t.Fatal("GetRAGTemplate() returned nil")
			}

			// Verify structure
			if template.Header == "" {
				t.Error("GetRAGTemplate() header is empty")
			}

			// Verify language-specific content
			built := template.Build()
			if tt.lang == MODEL_BASE_LANGUAGE_CN {
				if !strings.Contains(built, "检索增强生成") {
					t.Error("CN RAG template does not contain expected CN content")
				}
			} else {
				if !strings.Contains(built, "Retrieval Augmented Generation") {
					t.Error("EN RAG template does not contain expected EN content")
				}
			}
		})
	}
}

func TestPromptManager_GetSummaryTemplate(t *testing.T) {
	tests := []struct {
		name   string
		config *PromptConfig
		lang   string
	}{
		{
			name:   "CN template with default",
			config: nil,
			lang:   MODEL_BASE_LANGUAGE_CN,
		},
		{
			name: "CN template with custom config",
			config: &PromptConfig{
				ChatSummary: "Custom summary prompt",
			},
			lang: MODEL_BASE_LANGUAGE_CN,
		},
		{
			name:   "EN template",
			config: nil,
			lang:   MODEL_BASE_LANGUAGE_EN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewPromptManager(tt.config, MODEL_BASE_LANGUAGE_CN)
			template := pm.GetSummaryTemplate(tt.lang)

			if template == nil {
				t.Fatal("GetSummaryTemplate() returned nil")
			}

			// Verify structure
			if template.Header == "" {
				t.Error("GetSummaryTemplate() header is empty")
			}
			if template.Body == "" {
				t.Error("GetSummaryTemplate() body is empty")
			}

			// Verify config override works
			if tt.config != nil && tt.config.ChatSummary != "" {
				if !strings.Contains(template.Body, tt.config.ChatSummary) {
					t.Error("GetSummaryTemplate() does not include config ChatSummary")
				}
			}
		})
	}
}

func TestPromptManager_GetEnhanceQueryTemplate(t *testing.T) {
	tests := []struct {
		name   string
		config *PromptConfig
		lang   string
	}{
		{
			name:   "CN template with default",
			config: nil,
			lang:   MODEL_BASE_LANGUAGE_CN,
		},
		{
			name: "CN template with custom config",
			config: &PromptConfig{
				EnhanceQuery: "Custom enhance query prompt",
			},
			lang: MODEL_BASE_LANGUAGE_CN,
		},
		{
			name:   "EN template",
			config: nil,
			lang:   MODEL_BASE_LANGUAGE_EN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewPromptManager(tt.config, MODEL_BASE_LANGUAGE_CN)
			template := pm.GetEnhanceQueryTemplate(tt.lang)

			if template == nil {
				t.Fatal("GetEnhanceQueryTemplate() returned nil")
			}

			// Verify structure
			if template.Header == "" {
				t.Error("GetEnhanceQueryTemplate() header is empty")
			}
			if template.Body == "" {
				t.Error("GetEnhanceQueryTemplate() body is empty")
			}

			// Verify config override works
			if tt.config != nil && tt.config.EnhanceQuery != "" {
				if !strings.Contains(template.Body, tt.config.EnhanceQuery) {
					t.Error("GetEnhanceQueryTemplate() does not include config EnhanceQuery")
				}
			}
		})
	}
}

func TestPromptManager_GetButlerTemplate(t *testing.T) {
	pm := NewPromptManager(nil, MODEL_BASE_LANGUAGE_CN)
	template := pm.GetButlerTemplate(MODEL_BASE_LANGUAGE_CN)

	if template == nil {
		t.Fatal("GetButlerTemplate() returned nil")
	}

	// Verify structure
	if template.Header == "" {
		t.Error("GetButlerTemplate() header is empty")
	}
}

func TestPromptManager_GetJournalTemplate(t *testing.T) {
	pm := NewPromptManager(nil, MODEL_BASE_LANGUAGE_CN)
	template := pm.GetJournalTemplate(MODEL_BASE_LANGUAGE_CN)

	if template == nil {
		t.Fatal("GetJournalTemplate() returned nil")
	}

	// Verify structure
	if template.Header == "" {
		t.Error("GetJournalTemplate() header is empty")
	}
}

func TestPromptManager_ConfigurationPriority(t *testing.T) {
	// Test that configuration takes priority over system defaults
	customHeader := "Custom Global Header"
	config := &PromptConfig{
		Header: customHeader,
	}

	pm := NewPromptManager(config, MODEL_BASE_LANGUAGE_CN)
	template := pm.GetChatTemplate(MODEL_BASE_LANGUAGE_CN, nil)

	// Verify custom header from config is used
	if template.Header != customHeader {
		t.Errorf("Configuration priority failed, got header %v, want %v", template.Header, customHeader)
	}
}

func TestPromptManager_CommonVarsSet(t *testing.T) {
	pm := NewPromptManager(nil, MODEL_BASE_LANGUAGE_CN)
	template := pm.GetChatTemplate(MODEL_BASE_LANGUAGE_CN, nil)

	// Verify common variables are set
	if template.Vars[PROMPT_VAR_SITE_TITLE] == "" {
		t.Error("Common var SITE_TITLE not set")
	}
	if template.Vars[PROMPT_VAR_TIME_RANGE] == "" {
		t.Error("Common var TIME_RANGE not set")
	}
	if template.Vars[PROMPT_VAR_SYMBOL] == "" {
		t.Error("Common var SYMBOL not set")
	}
}

func TestPromptManager_LanguageSupport(t *testing.T) {
	pm := NewPromptManager(nil, MODEL_BASE_LANGUAGE_CN)

	tests := []struct {
		name    string
		getFunc func(string) *PromptTemplate
		lang    string
	}{
		{
			name:    "chat CN",
			getFunc: func(lang string) *PromptTemplate { return pm.GetChatTemplate(lang, nil) },
			lang:    MODEL_BASE_LANGUAGE_CN,
		},
		{
			name:    "chat EN",
			getFunc: func(lang string) *PromptTemplate { return pm.GetChatTemplate(lang, nil) },
			lang:    MODEL_BASE_LANGUAGE_EN,
		},
		{
			name:    "RAG CN",
			getFunc: func(lang string) *PromptTemplate { return pm.GetRAGTemplate(lang, nil) },
			lang:    MODEL_BASE_LANGUAGE_CN,
		},
		{
			name:    "RAG EN",
			getFunc: func(lang string) *PromptTemplate { return pm.GetRAGTemplate(lang, nil) },
			lang:    MODEL_BASE_LANGUAGE_EN,
		},
		{
			name:    "summary CN",
			getFunc: pm.GetSummaryTemplate,
			lang:    MODEL_BASE_LANGUAGE_CN,
		},
		{
			name:    "summary EN",
			getFunc: pm.GetSummaryTemplate,
			lang:    MODEL_BASE_LANGUAGE_EN,
		},
		{
			name:    "enhance_query CN",
			getFunc: pm.GetEnhanceQueryTemplate,
			lang:    MODEL_BASE_LANGUAGE_CN,
		},
		{
			name:    "enhance_query EN",
			getFunc: pm.GetEnhanceQueryTemplate,
			lang:    MODEL_BASE_LANGUAGE_EN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := tt.getFunc(tt.lang)
			if template == nil {
				t.Fatal("Template is nil")
			}
			if template.Lang != tt.lang {
				t.Errorf("Language mismatch, got %v, want %v", template.Lang, tt.lang)
			}

			// Verify template can build without errors
			built := template.Build()
			if built == "" {
				t.Error("Built template is empty")
			}

			// Verify language-specific content
			built = template.Build()
			if tt.lang == MODEL_BASE_LANGUAGE_CN {
				if !strings.Contains(built, "Markdown 语法说明") {
					t.Error("CN template does not contain expected CN content")
				}
			} else {
				if !strings.Contains(built, "system supports Markdown") {
					t.Error("EN template does not contain expected EN content")
				}
			}
		})
	}
}

func TestPromptManager_GetRAGToolResponseTemplate(t *testing.T) {
	tests := []struct {
		name string
		lang string
	}{
		{
			name: "CN template",
			lang: MODEL_BASE_LANGUAGE_CN,
		},
		{
			name: "EN template",
			lang: MODEL_BASE_LANGUAGE_EN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewPromptManager(nil, MODEL_BASE_LANGUAGE_CN)
			template := pm.GetRAGToolResponseTemplate(tt.lang, nil)

			if template == nil {
				t.Fatal("GetRAGToolResponseTemplate() returned nil")
			}

			// Verify template has no header or append
			if template.Header != "" {
				t.Error("GetRAGToolResponseTemplate() should have empty header")
			}
			if template.Append != "" {
				t.Error("GetRAGToolResponseTemplate() should have empty append")
			}

			// Verify body is set
			if template.Body == "" {
				t.Error("GetRAGToolResponseTemplate() body is empty")
			}

			// Verify language-specific content
			built := template.Build()
			if tt.lang == MODEL_BASE_LANGUAGE_CN {
				if !strings.Contains(built, "知识库检索结果") {
					t.Error("CN RAG tool response template does not contain expected CN content")
				}
				if !strings.Contains(built, "使用指南") {
					t.Error("CN RAG tool response template missing usage guidelines")
				}
			} else {
				if !strings.Contains(built, "Knowledge Base Search Results") {
					t.Error("EN RAG tool response template does not contain expected EN content")
				}
				if !strings.Contains(built, "Usage Guidelines") {
					t.Error("EN RAG tool response template missing usage guidelines")
				}
			}

			// Verify variables can be set
			template.SetVar("${knowledge_count}", "5")
			template.SetVar("relevant_passage", "test content")
			built = template.Build()

			if !strings.Contains(built, "5") {
				t.Error("knowledge_count variable not replaced")
			}
			if !strings.Contains(built, "test content") {
				t.Error("relevant_passage variable not replaced")
			}
		})
	}
}
