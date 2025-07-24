package ai

const PROMPT_NAMED_SESSION_DEFAULT_CN = `请通过用户对话内容分析该对话的主题，尽可能简短，限制在20个字以内，不要以标点符合结尾。请使用用户使用的语言(中文，英文，或其他语言)进行命名。`
const PROMPT_NAMED_SESSION_DEFAULT_EN = `Please analyze the conversation's topic based on the user's dialogue, keeping it concise and within 20 words without punctuation.`

const PROMPT_SUMMARY_DEFAULT_CN = `请总结以下用户对话，作为后续聊天的上下文信息。`
const PROMPT_SUMMARY_DEFAULT_EN = `Please summarize the following user conversation as contextual information for future chats.`

const PROMPT_PROCESS_CONTENT_CN = `
请帮助我对以下用户输入的文本进行预处理。目标是提高文本的质量，以便于后续的embedding处理。请遵循以下步骤：

清洗文本：去除特殊字符和多余空格，标准化文本（如小写化）。
分块：将较长的文本分成句子或小段落，以便更好地捕捉语义。
摘要：提取文本中的关键信息，生成简短的摘要。
增加上下文信息：结合相关的元数据（如主题、时间等），并在文本开头添加标签。
标签提取：最多提取5个，至少提取2个。

如果用户提供的内容中有出现对时间的描述，请尽可能将语义化的时间转换为对应的日期。
请在处理后提供清洗后的文本、分块结果、摘要以及添加上下文信息后的最终文本作为整体总结内容。
注意：无论是清洗还是分块，你只需要回答不重复的内容，并且不必告诉用户这是清洗内容，那是分块内容。
你可以结合以下基于现在的时间表来理解用户的内容：
${time_range}
此外参考内容中可能出现的一些系统语法，你可以忽略这些标识，把它当成一个字符串整体：
${symbol}
`

const PROMPT_PROCESS_CONTENT_EN = `
Please help preprocess the following user-input text to improve its quality for embedding purposes. Follow these steps:
1.Clean the Text: Remove special characters and extra spaces, and standardize the text (e.g., lowercase).
2.Chunking: Divide longer text into sentences or small paragraphs to better capture semantic meaning.
3.Summarization: Extract key information from the text to create a concise summary.
4.Add Contextual Information: Incorporate relevant metadata (such as topic, date), adding tags at the beginning of the text.
5.Tag Extraction: Extract between 2 to 5 tags.
If needed, you may organize the user content from multiple perspectives.
If the user’s content contains time descriptions, convert any semantic time expressions to specific dates whenever possible.
After processing, provide the cleaned text, chunked result, summary, and the final text with contextual information as a comprehensive output.
Note: For cleaning and chunking, respond only with unique information and avoid labeling sections as "cleaned text" or "chunked content."
You can use the current timeline to better understand the user's content: 
${time_range}
Additionally, some system syntax may appear in the reference content. You can ignore these markers and treat them as a single string: 
${symbol}
`

const PROMPT_CHUNK_CONTENT_CN = `
你是一位RAG技术专家，你需要将用户提供的内容进行分块处理(chunk)，你只对用户提供的内容做分块处理，用户并不是在与你聊天。
将内容分块的原因是希望embedding的结果与用户之后的搜索词匹配度能够更高，如果你认为用户提供的内容已经足够精简，则可以直接使用原文作为一个块。
请结合文章整体内容来对用户内容进行分块，一定不能疏漏与块相关的上下文信息，例如时间点、节日、日期、什么技术等。你的目的不是为了缩减内容的长度，而是将原本表达几个不同内容的长文转换为一个个独立内容块。
注意：分块一定不能缺乏上下文信息，不能出现主语不明确的语句，分块后你要将分块后的内容与用户提供的原文进行语义比较，看分块内容与原文对应的部分所表达的意思是否相同，不同则需要重新生成。
至少生成1个块，至多生成10个块。
至多提取5个标签。

### 分块处理过程

1. **解析内容**：首先，理解整个文本的上下文和结构。
2. **识别关键概念**：找出文本中的重要术语、方法、流程等。
3. **生成描述**：为每个分块提供详细的描述，说明其在整体内容中的位置和意义。

### 错误的例子
"将这些事情做完"，这样的结果丢失了上下文，用户会不清楚"这些"指的是什么。
避免出现代码与知识点分离的情况，这样既不知道代码想要表示的意思，也不知道知识点具体的实现是什么样的。

### 检查
分块结束后，重新检查所有分块，是否与用户所描述内容相关，若不相关则删除该分块。

你可以结合以下基于现在的时间表来理解用户的内容：
${time_range}
此外参考内容中可能出现的一些系统语法，你可以忽略这些标识，把它当成一个字符串整体：
${symbol}
`

const PROMPT_CHUNK_CONTENT_EN = `
You are a RAG technology expert, and you need to chunk the content provided by the user. Your focus is solely on chunking the user's content; the user is not engaged in a conversation with you.
The purpose of chunking the content is to improve the matching of embedding results with the user's future search terms. If you believe the content is already concise enough, you can use the original text as a single chunk.
Please consider the overall context of the text when chunking the user's content. Ensure that no relevant contextual information, such as time points, holidays, dates, or specific technologies, is overlooked. Your goal is not to shorten the content, but to transform a longer text that expresses several different ideas into distinct, independent content blocks.
Note: Each chunk must retain contextual information and should not contain ambiguous statements. After chunking, compare the chunks with the original user content to ensure the meanings align; if they differ, regenerate the chunks.
Generate at least 1 chunk and a maximum of 10 chunks, along with up to 5 tags.

### Chunking Process
1. **Analyze Content**: First, understand the overall context and structure of the text.
2. **Identify Key Concepts**: Find important terms, methods, processes, etc., within the text.
3. **Generate Descriptions**: Provide detailed descriptions for each chunk, explaining its position and significance in the overall content.

### Example of Incorrect Chunking
"Complete these tasks," which loses context and leaves the user unclear about what "these" refers to. Avoid separating code from the knowledge points; otherwise, the meaning of the code and its specific implementation will be lost.

### Review
After chunking, recheck all chunks to ensure they are relevant to the user's described content. If not, remove that chunk.

You can refer to the current timeline to better understand the user's content:
${time_range}

Additionally, some system syntax may appear in the reference content. You can ignore these markers and treat them as a single string:
${symbol}
`

// 首先需要明确，参考内容中使用$hidden[]包裹起来的内容是用户脱敏后的内容，你无需做特殊处理，如果需要原样回答即可
//
//	例如参考文本为：XXX事项涉及用户为$hidden[user1]。
//	你在回答时如果需要回答该用户，可以直接回答“$hidden[user1]”
const GENERATE_PROMPT_TPL_CN = GENERATE_PROMPT_TPL_NONE_CONTENT_CN + `
我先给你提供一个时间线的参考：
${time_range}
你需要结合上述时间线来理解我问题中所提到的时间(如果有)。
以下是我记录的一些“参考内容”，这些内容都是历史记录，请不要将参考内容中提到的时间以为是基于现在发生的：
--------------------------------------
${relevant_passage}
--------------------------------------
你需要结合“参考内容”来回答用户的提问，
注意，“参考内容”中可能有部分内容描述的是同一件事情，但是发生的时间不同，当你无法选择应该参考哪一天的内容时，可以结合用户提出的问题进行分析。
如果你从“参考内容”中找到了我想要的答案，可以告诉我你参考了哪些内容的ID，并尽可能地将参考内容中相关的图片、音视频也一同告诉我(URL等)。
以下是参考内容中可能出现的一些系统语法，你可以忽略这些标识，把它当成一个字符串整体：
${symbol}
Markdown中有些内容是通过HTML标签表示的，请不要额外处理这些HTML标签，例如<video>等，它们都是系统语法，请不要语义化这些内容。
在回答时请提前组织好语言，不要反复出现重复的内容。
用户使用什么语言与你沟通，你就使用什么语言回复用户，如果你不会该语言则使用英语来与用户交流。
`

const GENERATE_PROMPT_TPL_EN = GENERATE_PROMPT_TPL_NONE_CONTENT_EN + `
Here’s a reference timeline I’m providing: 
${time_range}
You need to use the timeline above to understand any mentioned time in my question (if applicable).
Below are some "reference materials" that include historical records. Please do not assume that the times mentioned in the reference content are based on current events:
{relevant_passage}
Please use the "reference materials" to answer my questions.
Note that some parts of the "reference materials" may describe the same event but with different timestamps. When you're unsure which date to use, analyze the context of my question to choose accordingly.
If you find the answer within the "reference materials," let me know which content IDs you used as references. Please also provide me with any associated images, audio, and video from the related content, including URLs if possible.
Please respond in Markdown format using the same language as my question.
Below are some system syntax symbols that may appear in the reference content. You can ignore these, treating them as strings without semantic interpretation: 
${symbol}
You must respond in the language used by the user in their most recent question. If you are not proficient in that language, you may respond in English.
`

const GENERATE_PROMPT_TPL_NONE_CONTENT_CN = `
你是一位RAG助理，名字叫做Quka，模型为Quka Engine。
你需要以Markdown的格式回复用户。
`

const IMAGE_GENERATE_PROMPT_CN = `
请帮我分析出图片中的重要信息，使用一段话告诉我。
一定要使用 ${lang} 来进行回复。
`

const IMAGE_GENERATE_PROMPT_EN = `
Please help me analyze the important information in the image and summarize it in one sentence.
Please answer me using the ${lang} language.
`

const GENERATE_PROMPT_TPL_NONE_CONTENT_EN = `You are an RAG assistant named Quka, and your model is Quka Engine. You need to respond to users in Markdown format.`
