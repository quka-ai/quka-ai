package utils

import (
	"encoding/json"
	"testing"

	"github.com/davidscottmills/goeditorjs"
	"github.com/stretchr/testify/assert"
)

func TestConvertEditorJSBlocksToMarkdown(t *testing.T) {
	SetupGlobalEditorJS("https://static.example.com")
	blocksString := json.RawMessage(`{"time":1731487512437,"blocks":[{"id":"vTlW-R-6WB","type":"paragraph","data":{"text":"About RAG  "}},{"id":"ZrnMw-Qfpo","type":"paragraph","data":{"text":"Retrieval-Augmented Generation (RAG) is a technique that enhances language model capabilities by integrating information retrieval with text generation. In RAG systems, rather than relying solely on a model's internal knowledge, external data sources are accessed to provide relevant information on demand, making responses more accurate and contextually informed."}},{"id":"GPiOg8aMSN","type":"paragraph","data":{"text":"Here’s how it works:"}},{"id":"0hb_hIwoUD","type":"list","data":{"style":"unordered","items":["Retrieval Step: When a question or prompt is provided, the RAG system first performs a search or query over a large knowledge base (e.g., documents, databases, or web content) to retrieve relevant information.  ","Generation Step: The retrieved information is then passed to a language model, which generates a response that incorporates the most relevant context.  "]}},{"id":"xhImtEtFD5","type":"paragraph","data":{"text":"This technique is particularly valuable for applications where up-to-date, domain-specific, or extensive knowledge is required. RAG helps improve accuracy, reduces hallucinations (incorrect or invented information), and allows for the dynamic update of knowledge without needing to retrain the model itself."}}],"version":"2.30.7"}`)

	md, err := ConvertEditorJSRawToMarkdown(blocksString)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(md)
}

func TestConvertListv2(t *testing.T) {
	testData := `{
  "id": "H9yyjnIYeR",
  "type": "listv2",
  "data": {
    "style": "unordered",
    "meta": {},
    "items": [
      {
        "content": "aaaaa",
        "meta": {},
        "items": [
          {
            "content": "bbbbb",
            "meta": {},
            "items": [
              {
                "content": "33333111",
                "meta": {},
                "items": []
              }
            ]
          },
		  {
		  	"content": "ccccc"
		  }
        ]
      }
    ]
  }
}`
	var block goeditorjs.EditorJSBlock

	if err := json.Unmarshal([]byte(testData), &block); err != nil {
		t.Fatal(err)
	}
	handler := ListV2Handler{}
	result, err := handler.GenerateMarkdown(block)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "- aaaaa  \n  - bbbbb  \n    - 33333111  \n  - ccccc  ", result)

	result, err = handler.GenerateHTML(block)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "<ul><li><span>aaaaa</span><ul><li><span>bbbbb</span><ul><li><span>33333111</span></li></ul></li><li>ccccc</li></ul></li></ul>", result)
}
