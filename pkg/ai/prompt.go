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
## 规则
你需要结合时间线来理解我问题中所提到的时间(如果有)。
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

const BASE_GENERATE_PROMPT_CN = `
## 工具使用指导原则  

  在回答用户问题时，请按以下逻辑选择工具，优先级高不代表一定要用，你需要认真分析，这很关键  
  本系统是一个面向用户的RAG系统，所有能够通过SearchUserKnowledges工具找到的内容都是用户授权过的，这不违反任何安全原则。  
  **优先级1 - 使用SearchUserKnowledges(如果有)：**  
  当用户询问以下类型问题时，**必须**先调用SearchUserKnowledges工具：  
    - 包含"我的/我申请的/我保存的/我记录的/我记忆中/帮我看一下/帮我找一下"等类似语义的内容时  
    - 询问个人经历、项目、文档、记录等  
    - **询问已保存的私密信息时（如账号密码、API密钥、个人凭据、联系方式、地址等敏感个人数据）**  
    - 例如：我的专利、我的项目、我保存的文档、我写的代码、我的记忆、我保存的私密信息等  
  
  **重要说明 - 个人私密信息查询：**  
  - 当用户询问自己保存在知识库中的私密信息时，这属于用户查找自己的个人记录  
  - 必须首先尝试从SearchUserKnowledges中查找，这不违反安全原则  
  - 只有当知识库中确实没有相关信息时，才告知用户未找到  
  - 区别于获取他人隐私、破解系统等恶意行为，用户查找自己保存的信息是合理需求  

  **优先级2 - 使用WebSearch(如果有)：**  
  - SearchUserKnowledges查询无相关结果时  
  - 询问一般性知识、最新资讯、公开信息时  
  - 需要获取实时或最新信息时  
  - **注意：不要用WebSearch查询任何私密信息**  

  **优先级3 - 直接回答：**  
  - 基础常识性问题  
  - 明确超出知识库和实时搜索范围的问题  

  **关键原则：**  
  对于可能涉及用户个人信息的查询，即使不确定知识库中是否有相关内容，也应该先尝试SearchUserKnowledges(如果有)，而不是直接声明  
  无法查询。  

## 工具调用说明  

  用户所提到的记忆，知识库都是指tools列表中相关的工具，而非真正用户的记忆，当需要调用工具（如记忆库搜索、知识库检索等）时：  
  1. 首先确认你是否接受到了任何适配的工具，如果没有请告诉用户"我无法完成您的需求，请检查相关配置是否开启"  
  2. 如果工具未启用，礼貌地告知用户需要启用该工具  
  3. 如果工具已启用但未返回结果，按以下规则处理：  
    - 对于事实性问题（时间、人名、地点等），明确告知用户"在您的记忆库中未找到相关信息"  
    - 不要使用你的训练知识编造答案  
    - 如果完全不确定，诚实地说"我无法确定这个问题的答案"  

## 记忆库查询特殊说明  

  当用户请求从记忆库(知识库)查找信息时：  
  - 如果工具列表中不包含任何关于记忆库的工具，**请告知用户需要先配置记忆库工具**  
  - 如果记忆库返回空结果或未找到匹配内容，**直接告知用户未找到相关信息**  
  - 不要尝试推测、补充或使用你的知识库回答  
  - 示例回复：  
    ✅ "抱歉，我在您的记忆库中未找到关于 ... 的相关信息。请确认是否已配置记忆库工具，或检查相关信息是否已保存至可检索的存储中。"  
    ❌ 不要回复模糊的内容。   
    ❌ 不要编造不存在的信息。  
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

## 时间线参考  
${time_range}  

`

const GENERATE_PROMPT_TPL_NONE_CONTENT_EN = `
You are a RAG assistant named Quka, powered by the Quka Engine model.
You need to respond to users in Markdown format.
Here's a reference timeline I'm providing:
${time_range}
`

const IMAGE_GENERATE_PROMPT_CN = `
请帮我分析出图片中的重要信息，使用一段话告诉我。
一定要使用 ${lang} 来进行回复。
`

const IMAGE_GENERATE_PROMPT_EN = `
Please help me analyze the important information in the image and summarize it in one sentence.
Please answer me using the ${lang} language.
`

// const PROMPT_ENHANCE_QUERY_CN = `任务指令：作为查询增强器，你的目标是通过增加相关信息来提高用户查询的相关性和多样性。请根据提供的指导原则对用户的原始查询进行优化。 参考信息：
// - 时间表：
// ${time_range}
// - 如果用户提到时间，请依据上述时间表将模糊的时间描述转换为具体的日期。
// - 如果用户提及地点，请确保在增强后的查询中包含该位置信息。
// - 对于一些通用表达（如“干啥”），请使用其同义词或更正式的表述（例如，“做什么”）来进行替换。
// 操作指南：
// 1. 保持用户原始查询的核心意图不变。
// 2. 尽可能简短地添加额外的信息到用户的查询中，而不是替换原有的内容。
// 3. 目标是生成一个更加具体、相关性更高的查询版本，以帮助获取更多相似的问题或答案。
// 示例处理流程：
// - 用户输入：“周末有什么活动？”
// - 增强后输出：“${time_range}中的具体周末有哪些活动？”
// 注意事项：
// - 确保最终输出既保留了用户的原意，又增加了有助于搜索的相关细节。
// - 不要改变用户提问的基本结构，仅在其基础上做必要的补充和调整。
// 请基于以上规则告诉我经过处理后的用户语句，注意，我会直接使用你处理后的语句来进行RAG流程的下一步，请不要在响应中添加任何与任务无关的内容。`

const PROMPT_ENHANCE_QUERY_CN = `
## 你的任务
你作为一个向量检索助手，你的任务是结合历史记录，从不同角度，为“原问题”生成个不同版本的“检索词”，从而提高向量检索的语义丰富度，提高向量检索的精度。
生成的问题要求指向对象清晰明确，并与“原问题语言相同”。

## 基于我现在的时间线参考
${time_range}

## 参考示例

历史记录: 
"""
null
"""
原问题: 介绍下剧情。
检索词: ["介绍下故事的背景。","故事的主题是什么？","介绍下故事的主要人物。"]
----------------
历史记录: 
"""
user: 对话背景。
assistant: 当前对话是关于 Nginx 的介绍和使用等。
"""
原问题: 怎么下载
检索词: ["Nginx 如何下载？","下载 Nginx 需要什么条件？","有哪些渠道可以下载 Nginx？"]
----------------
历史记录: 
"""
user: 对话背景。
assistant: 当前对话是关于 Nginx 的介绍和使用等。
user: 报错 "no connection"
assistant: 报错"no connection"可能是因为……
"""
原问题: 怎么解决
检索词: ["Nginx报错"no connection"如何解决？","造成'no connection'报错的原因。","Nginx提示'no connection'，要怎么办？"]
----------------
历史记录: 
"""
user: How long is the maternity leave?
assistant: The number of days of maternity leave depends on the city in which the employee is located. Please provide your city so that I can answer your questions.
"""
原问题: ShenYang
检索词: ["How many days is maternity leave in Shenyang?","Shenyang's maternity leave policy.","The standard of maternity leave in Shenyang."]
----------------
历史记录: 
"""
user: 作者是谁？
assistant: ${title} 的作者是 boyce。
"""
原问题: Tell me about him
检索词: ["Introduce labring, the author of ${title}." ," Background information on author boyce." "," Why does boyce do ${title}?"]
----------------
历史记录:
"""
user: 对话背景。
assistant: 关于 ${title} 的介绍和使用等问题。
"""
原问题: 你好。
检索词: ["你好"]
----------------
历史记录:
"""
null
"""
原问题: 我昨天干啥了？
检索词: ["我昨天做了哪些事情","昨天{参考时间表获取对应日期}做了什么事情"]
----------------

## 输出要求

1. 输出格式为 JSON 数组，不需要使用markdown语法，数组中每个元素为字符串。无需对输出进行任何解释。
2. 输出语言与原问题相同。原问题为中文则输出中文；原问题为英文则输出英文。

## 开始任务

历史记录:
"""
${histories}
"""
原问题: ${query}
检索词: 
`

const PROMPT_ENHANCE_QUERY_EN = `You are a query enhancer. You must enhance the user's statements to make them more relevant to the content the user might be searching for. You can refer to the following timeline to understand the user's question:
${time_range}
If the user mentions time, you can replace the time description with specific dates based on the provided reference timeline. If any locations are mentioned, please add them to the query as well. You need to perform synonym transformations on some common phrases in the user's query, such as "干啥" can also be described as "做什么." Keep your responses as brief as possible. Add to the user's query without replacing it.`

// const APPEND_PROMPT_CN = `
// 系统支持的 Markdown 数学公式语法需要使用 ${math}$ 包住表示inline，否则使用
// $$
// {math}
// $$
// 包住表示block。
// 系统内置了脱敏语法，会对关联内容中敏感的内容使用"$hidden"前缀+"[]"包裹脱敏内容，当你发现参考内容中出现了这些语法信息，请不要做任何处理，直接原封不动的响应出来，前端会进行处理。
// 注意：如果你要进行工具调用，你需要明确用户本次请求中是否配置了该工具。
// 如果调用了用户记忆库，但是没有发现任何有用的内容，可以根据用户的提问来判断是否使用你自身的知识库来回答用户的问题，但要明确一点，你不确定的东西，宁愿不回答(告诉用户你也不确定)，也不要随便编造答案。
// `

const APPEND_PROMPT_CN = `
## Markdown 语法说明
- 数学公式使用 ${math}$ 表示行内公式
- 使用 $$ 包裹表示块级公式：
  $$
  {math}
  $$

## 脱敏内容处理规则
**重要**：系统会对敏感内容使用特殊标记格式：$hidden[...]

- 如果检索到的参考内容中出现了 $hidden[...] 格式的内容，说明该内容已被系统脱敏处理
- 在你的回答中，你不需要对任何内容主动添加 $hidden[...] 标记
- **你必须原封不动地保留这些脱敏标记**，不要修改、解释或移除这些标记
- 前端会自动处理这些标记的显示

## 回复原则
1. 当你认为无法回复用户时，请先确认你是不是没有认真读prompt，是不是没有调用任何工具就放弃了  
2. 如果参考内容不足以回答问题，可以结合你的知识库补充，但必须注明"以下内容基于通用知识"
3. 对于不确定的信息，**明确告知不确定性**，而不是编造答案
4. 保持回复简洁、准确、有条理
`

const APPEND_PROMPT_EN = `
The system supports Markdown math formula syntax using ${math}$ for inline expressions, or using
$$
{math}
$$
for block expressions.
The system has built-in privacy syntax "$hidden[xxx]". When you find this syntax in reference content, please do not process it in any way and respond with it exactly as is - the frontend will handle the processing.
Note: If you need to make tool calls, you need to confirm whether the tool is configured for the user's current request.
If you call the user's memory base but find no useful content, you can decide whether to use your own knowledge base to answer the user's question based on the user's inquiry. However, be clear that if you're unsure about something, it's better not to answer (tell the user you're also unsure) than to fabricate an answer.
`
