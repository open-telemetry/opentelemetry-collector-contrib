// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapters

var langchainSimpleChatSpan = map[string]any{
	"openinference.span.kind":               "LLM",
	"llm.model_name":                        "anthropic.claude-3-haiku-20240307-v1:0",
	"llm.provider":                          "amazon_bedrock",
	"llm.invocation_parameters":             `{"model_id": "anthropic.claude-3-haiku-20240307-v1:0", "provider": "anthropic", "stream": false, "_type": "amazon_bedrock_chat", "stop": null}`,
	"llm.input_messages.0.message.role":     "user",
	"llm.input_messages.0.message.content":  "Say hello in one word",
	"llm.output_messages.0.message.role":    "assistant",
	"llm.output_messages.0.message.content": "Hello.",
	"llm.token_count.prompt":                int64(12),
	"llm.token_count.completion":            int64(5),
	"llm.token_count.total":                 int64(17),
	"input.value":                           `{"messages": [[{"lc": 1, "type": "constructor", "id": ["langchain", "schema", "messages", "HumanMessage"], "kwargs": {"content": "Say hello in one word", "type": "human"}}]]}`,
	"input.mime_type":                       "application/json",
	"output.value":                          `{"generations": [[{"text": "Hello.", "type": "ChatGeneration"}]]}`,
	"output.mime_type":                      "application/json",
	"metadata":                              `{"ls_provider": "amazon_bedrock", "ls_model_name": "anthropic.claude-3-haiku-20240307-v1:0", "ls_model_type": "chat"}`,
}

var langchainSystemMessageSpan = map[string]any{
	"openinference.span.kind":               "LLM",
	"llm.model_name":                        "anthropic.claude-3-haiku-20240307-v1:0",
	"llm.provider":                          "amazon_bedrock",
	"llm.input_messages.0.message.role":     "system",
	"llm.input_messages.0.message.content":  "You are a pirate.",
	"llm.input_messages.1.message.role":     "user",
	"llm.input_messages.1.message.content":  "Say hello in one word",
	"llm.output_messages.0.message.role":    "assistant",
	"llm.output_messages.0.message.content": "Ahoy!",
	"llm.token_count.prompt":                int64(18),
	"llm.token_count.completion":            int64(7),
	"llm.token_count.total":                 int64(25),
	"input.mime_type":                       "application/json",
	"output.mime_type":                      "application/json",
	"metadata":                              `{"ls_provider": "amazon_bedrock"}`,
}

var langchainToolCallOutputSpan = map[string]any{
	"openinference.span.kind":              "LLM",
	"llm.model_name":                       "anthropic.claude-3-haiku-20240307-v1:0",
	"llm.provider":                         "amazon_bedrock",
	"llm.input_messages.0.message.role":    "user",
	"llm.input_messages.0.message.content": "What's the weather in Seattle?",
	"llm.output_messages.0.message.role":   "assistant",
	"llm.output_messages.0.message.tool_calls.0.tool_call.function.name":      "get_weather",
	"llm.output_messages.0.message.tool_calls.0.tool_call.function.arguments": `{"city": "Seattle"}`,
	"llm.output_messages.0.message.tool_calls.0.tool_call.id":                 "toolu_bdrk_01GdVCiXNUBY9jGNhN45bFrU",
	"llm.token_count.prompt":       int64(332),
	"llm.token_count.completion":   int64(53),
	"llm.token_count.total":        int64(385),
	"llm.tools.0.tool.json_schema": `{"name": "get_weather", "description": "Get the weather for a city.", "input_schema": {"properties": {"city": {"type": "string"}}, "required": ["city"], "type": "object"}}`,
	"llm.invocation_parameters":    `{"model_id": "anthropic.claude-3-haiku-20240307-v1:0", "stop": null, "tools": [{"name": "get_weather", "description": "Get the weather for a city."}]}`,
	"input.mime_type":              "application/json",
	"output.mime_type":             "application/json",
	"metadata":                     `{"ls_provider": "amazon_bedrock"}`,
}

var langchainLLMWithToolResultSpan = map[string]any{
	"openinference.span.kind":              "LLM",
	"llm.model_name":                       "anthropic.claude-3-haiku-20240307-v1:0",
	"llm.provider":                         "amazon_bedrock",
	"llm.input_messages.0.message.role":    "user",
	"llm.input_messages.0.message.content": "What's the weather in Seattle?",
	"llm.input_messages.1.message.role":    "assistant",
	"llm.input_messages.1.message.tool_calls.0.tool_call.function.name":      "get_weather",
	"llm.input_messages.1.message.tool_calls.0.tool_call.function.arguments": `{"city": "Seattle"}`,
	"llm.input_messages.1.message.tool_calls.0.tool_call.id":                 "toolu_bdrk_01GdVCiXNUBY9jGNhN45bFrU",
	"llm.input_messages.2.message.role":                                      "tool",
	"llm.input_messages.2.message.content":                                   "72F and sunny in Seattle",
	"llm.input_messages.2.message.name":                                      "get_weather",
	"llm.input_messages.2.message.tool_call_id":                              "toolu_bdrk_01GdVCiXNUBY9jGNhN45bFrU",
	"llm.output_messages.0.message.role":                                     "assistant",
	"llm.output_messages.0.message.content":                                  "The weather in Seattle is 72F and sunny.",
	"llm.token_count.prompt":                                                 int64(403),
	"llm.token_count.completion":                                             int64(15),
	"llm.token_count.total":                                                  int64(418),
	"llm.tools.0.tool.json_schema":                                           `{"name": "get_weather", "description": "Get the weather for a city.", "input_schema": {"properties": {"city": {"type": "string"}}, "required": ["city"], "type": "object"}}`,
	"input.mime_type":                                                        "application/json",
	"output.mime_type":                                                       "application/json",
}

var langchainToolSpan = map[string]any{
	"openinference.span.kind": "TOOL",
	"tool.name":               "get_weather",
	"tool.description":        "Get the weather for a city.",
	"input.value":             "{'city': 'Seattle'}",
	"input.mime_type":         "application/json",
	"output.value":            `{"content": "72F and sunny in Seattle", "type": "tool", "name": "get_weather", "tool_call_id": "toolu_bdrk_01GdVCiXNUBY9jGNhN45bFrU", "status": "success"}`,
	"output.mime_type":        "application/json",
	"metadata":                `{"langgraph_step": 2, "langgraph_node": "tools"}`,
}

var langchainAgentSpan = map[string]any{
	"openinference.span.kind":              "AGENT",
	"input.value":                          `{"messages": [{"content": "What's the weather in Seattle?", "type": "human"}], "remaining_steps": 9999}`,
	"input.mime_type":                      "application/json",
	"output.value":                         `{"messages": [{"content": "", "additional_kwargs": {"usage": {"prompt_tokens": 332, "completion_tokens": 53, "cache_read_input_tokens": 0, "cache_write_input_tokens": 0, "total_tokens": 385}, "stop_reason": "tool_use", "model_id": "anthropic.claude-3-haiku-20240307-v1:0"}, "type": "ai"}]}`,
	"output.mime_type":                     "application/json",
	"llm.input_messages.0.message.role":    "user",
	"llm.input_messages.0.message.content": "What's the weather in Seattle?",
	"metadata":                             `{"langgraph_step": 1, "langgraph_node": "agent"}`,
}

var langchainChainOutputWithMetadataSpan = map[string]any{
	"openinference.span.kind":              "CHAIN",
	"input.mime_type":                      "application/json",
	"output.mime_type":                     "application/json",
	"output.value":                         `{"messages": [{"content": "The weather in Seattle is 72F and sunny.", "additional_kwargs": {"usage": {"prompt_tokens": 403, "completion_tokens": 15, "cache_read_input_tokens": 0, "cache_write_input_tokens": 0, "total_tokens": 418}, "stop_reason": "end_turn", "model_id": "anthropic.claude-3-haiku-20240307-v1:0"}, "type": "ai", "tool_calls": []}]}`,
	"llm.input_messages.0.message.role":    "user",
	"llm.input_messages.0.message.content": "What's the weather in Seattle?",
	"metadata":                             `{"langgraph_step": 3, "langgraph_node": "agent"}`,
}

var langchainChainOutputWithToolCallSpan = map[string]any{
	"openinference.span.kind": "CHAIN",
	"output.value":            `{"messages": [{"content": "", "additional_kwargs": {"usage": {"prompt_tokens": 332, "completion_tokens": 53, "cache_read_input_tokens": 0, "cache_write_input_tokens": 0, "total_tokens": 385}, "stop_reason": "tool_use", "model_id": "anthropic.claude-3-haiku-20240307-v1:0"}, "type": "ai", "tool_calls": [{"name": "get_weather", "args": {"city": "Seattle"}, "id": "toolu_bdrk_01GdVCiXNUBY9jGNhN45bFrU"}]}]}`,
	"output.mime_type":        "application/json",
}

var langchainChainToolsNodeSpan = map[string]any{
	"openinference.span.kind": "CHAIN",
	"input.value":             `{"__type": "tool_call_with_context", "tool_call": {"name": "get_clinic_hours", "args": {}, "id": "toolu_bdrk_01McWSdPrfhjsHiutBbjmgDK", "type": "tool_call"}, "state": {"messages": [{"content": "What are your hours?", "type": "human"}, {"content": "", "type": "ai", "tool_calls": [{"name": "get_clinic_hours", "args": {}, "id": "toolu_bdrk_01McWSdPrfhjsHiutBbjmgDK"}]}], "remaining_steps": 9999}}`,
	"input.mime_type":         "application/json",
	"output.value":            `{"messages": [{"content": "Monday-Friday: 8AM-6PM, Saturday: 9AM-4PM, Sunday: Closed.", "type": "tool", "name": "get_clinic_hours", "tool_call_id": "toolu_bdrk_01McWSdPrfhjsHiutBbjmgDK", "status": "success"}]}`,
	"output.mime_type":        "application/json",
}

var langchainChainOutputArrayFormatSpan = map[string]any{
	"openinference.span.kind": "CHAIN",
	"output.value":            `[{"content": "What's the weather in Seattle?", "type": "human", "name": null, "id": "1d29a593-acd7-4fc4-8f46-b9f4a3977053"}, {"content": "", "additional_kwargs": {"usage": {"prompt_tokens": 332, "completion_tokens": 53}, "stop_reason": "tool_use", "model_id": "anthropic.claude-3-haiku-20240307-v1:0"}, "type": "ai", "tool_calls": [{"name": "get_weather", "args": {"city": "Seattle"}, "id": "toolu_bdrk_01GdVCiXNUBY9jGNhN45bFrU"}]}]`,
	"output.mime_type":        "application/json",
}

var langchainPromptSpan = map[string]any{
	"openinference.span.kind":       "PROMPT",
	"input.value":                   `{"input_language": "English", "output_language": "French", "input": "Hello"}`,
	"input.mime_type":               "application/json",
	"output.value":                  `{"messages": [{"content": "Translate English to French.", "type": "system"}, {"content": "Hello", "type": "human"}]}`,
	"output.mime_type":              "application/json",
	"llm.prompt_template.template":  "Translate {input_language} to {output_language}.",
	"llm.prompt_template.variables": `{"input_language": "English", "output_language": "French"}`,
}

var langchainChainSpan = map[string]any{
	"openinference.span.kind": "CHAIN",
	"input.value":             `{"input_language": "English", "output_language": "French", "input": "Hello"}`,
	"input.mime_type":         "application/json",
	"output.value":            `{"content": "Bonjour", "additional_kwargs": {"usage": {"prompt_tokens": 14, "completion_tokens": 7, "total_tokens": 21}, "stop_reason": "end_turn", "model_id": "anthropic.claude-3-haiku-20240307-v1:0"}, "type": "ai"}`,
	"output.mime_type":        "application/json",
}

var langchainShouldContinueSpan = map[string]any{
	"openinference.span.kind":              "CHAIN",
	"input.mime_type":                      "application/json",
	"output.value":                         "__end__",
	"llm.input_messages.0.message.role":    "user",
	"llm.input_messages.0.message.content": "What's the weather in Seattle?",
	"metadata":                             `{"langgraph_step": 3, "langgraph_node": "agent"}`,
}

var langchainLangGraphTopLevelSpan = map[string]any{
	"openinference.span.kind":              "CHAIN",
	"input.value":                          `{"messages": [["user", "What's the weather in Seattle?"]]}`,
	"input.mime_type":                      "application/json",
	"llm.input_messages.0.message.role":    "user",
	"llm.input_messages.0.message.content": "What's the weather in Seattle?",
}

var bedrockConverseLLMSpan = map[string]any{
	"openinference.span.kind":           "LLM",
	"llm.model_name":                    "anthropic.claude-3-haiku-20240307-v1:0",
	"llm.invocation_parameters":         `{"maxTokens": 100, "temperature": 0.7}`,
	"llm.input_messages.0.message.role": "user",
	"llm.input_messages.0.message.contents.0.message_content.type": "text",
	"llm.input_messages.0.message.contents.0.message_content.text": "Say hello in one word",
	"llm.output_messages.0.message.role":                           "assistant",
	"llm.output_messages.0.message.content":                        "Hello.",
	"llm.token_count.prompt":                                       int64(12),
	"llm.token_count.completion":                                   int64(5),
	"llm.token_count.total":                                        int64(17),
	"input.value":                                                  "Say hello in one word",
	"output.value":                                                 "Hello.",
}

var bedrockConverseWithSystemSpan = map[string]any{
	"openinference.span.kind":           "LLM",
	"llm.model_name":                    "anthropic.claude-3-haiku-20240307-v1:0",
	"llm.invocation_parameters":         `{"maxTokens": 100, "temperature": 0.7}`,
	"llm.input_messages.0.message.role": "system",
	"llm.input_messages.0.message.contents.0.message_content.type": "text",
	"llm.input_messages.0.message.contents.0.message_content.text": "You are a pirate.",
	"llm.input_messages.1.message.role":                            "user",
	"llm.input_messages.1.message.contents.0.message_content.type": "text",
	"llm.input_messages.1.message.contents.0.message_content.text": "Say hello in one word",
	"llm.output_messages.0.message.role":                           "assistant",
	"llm.output_messages.0.message.content":                        "Ahoy!",
	"llm.token_count.prompt":                                       int64(18),
	"llm.token_count.completion":                                   int64(7),
	"llm.token_count.total":                                        int64(25),
	"input.value":                                                  "Say hello in one word",
	"output.value":                                                 "Ahoy!",
}

var bedrockConverseToolUseSpan = map[string]any{
	"openinference.span.kind":           "LLM",
	"llm.model_name":                    "anthropic.claude-3-haiku-20240307-v1:0",
	"llm.invocation_parameters":         `{"maxTokens": 200}`,
	"llm.input_messages.0.message.role": "user",
	"llm.input_messages.0.message.contents.0.message_content.type": "text",
	"llm.input_messages.0.message.contents.0.message_content.text": "What's the weather in Seattle?",
	"llm.output_messages.0.message.role":                           "assistant",
	"llm.token_count.prompt":                                       int64(331),
	"llm.token_count.completion":                                   int64(53),
	"llm.token_count.total":                                        int64(384),
	"input.value":                                                  "What's the weather in Seattle?",
}

var llamaindexLLMSpan = map[string]any{
	"openinference.span.kind":                    "LLM",
	"llm.model_name":                             "anthropic.claude-3-haiku-20240307-v1:0",
	"llm.invocation_parameters":                  `{"context_window":200000,"num_output":512,"is_chat_model":true,"is_function_calling_model":true,"model_name":"anthropic.claude-3-haiku-20240307-v1:0"}`,
	"llm.input_messages.0.message.role":          "user",
	"llm.input_messages.0.message.content":       "Say hello in one word",
	"llm.output_messages.0.message.role":         "assistant",
	"llm.output_messages.0.message.content":      "Hello.",
	"llm.token_count.prompt":                     int64(12),
	"llm.token_count.completion":                 int64(5),
	"llm.token_count.total":                      int64(17),
	"llm.token_count.prompt_details.cache_read":  int64(0),
	"llm.token_count.prompt_details.cache_write": int64(0),
	"input.value":                                `{"messages": ["ChatMessage(role=<MessageRole.USER: 'user'>, content='Say hello in one word')"]}`,
	"input.mime_type":                            "application/json",
	"output.value":                               "assistant: Hello.",
}

var llamaindexSystemMessageSpan = map[string]any{
	"openinference.span.kind":                    "LLM",
	"llm.model_name":                             "anthropic.claude-3-haiku-20240307-v1:0",
	"llm.invocation_parameters":                  `{"context_window":200000,"num_output":512,"is_chat_model":true,"is_function_calling_model":true,"model_name":"anthropic.claude-3-haiku-20240307-v1:0"}`,
	"llm.input_messages.0.message.role":          "system",
	"llm.input_messages.0.message.content":       "You are a pirate.",
	"llm.input_messages.1.message.role":          "user",
	"llm.input_messages.1.message.content":       "Say hello in one word",
	"llm.output_messages.0.message.role":         "assistant",
	"llm.output_messages.0.message.content":      "Ahoy!",
	"llm.token_count.prompt":                     int64(18),
	"llm.token_count.completion":                 int64(7),
	"llm.token_count.total":                      int64(25),
	"llm.token_count.prompt_details.cache_read":  int64(0),
	"llm.token_count.prompt_details.cache_write": int64(0),
	"input.mime_type":                            "application/json",
	"output.value":                               "assistant: Ahoy!",
}

var crewaiCrewKickoffSpan = map[string]any{
	"openinference.span.kind": "CHAIN",
	"crew_agents":             `[{"key": "dcb764bf53a0", "id": "f062a8ee-8a69", "role": "Researcher", "goal": "Answer questions accurately"}]`,
	"crew_id":                 "3d86fbf1-c04c-4a91-8478-610847238a84",
	"crew_inputs":             "",
	"crew_key":                "e0e467a06ac5abbbbfed25ac101c0909",
	"crew_tasks":              `[{"id": "d76fde56-6272", "description": "What is the capital of France?", "expected_output": "A single sentence.", "agent_role": "Researcher"}]`,
}

var crewaiCrewCreatedSpan = map[string]any{
	"crew_agents":           `[{"key": "dcb764bf53a0", "role": "Researcher", "verbose?": false, "max_iter": 25}]`,
	"crew_fingerprint":      "6d2eb9fd-85aa-4af2-9f0e-1630c72b06e6",
	"crew_id":               "3d86fbf1-c04c-4a91-8478-610847238a84",
	"crew_key":              "e0e467a06ac5abbbbfed25ac101c0909",
	"crew_memory":           "False",
	"crew_number_of_agents": "1",
	"crew_number_of_tasks":  "1",
	"crew_process":          "Process.sequential",
	"crew_tasks":            `[{"key": "ad4694b4e962", "id": "d76fde56-6272", "async_execution?": false, "human_input?": false, "agent_role": "Researcher"}]`,
	"crewai_version":        "1.9.1",
	"python_version":        "3.12.7",
}

var crewaiTaskCreatedSpan = map[string]any{
	"agent_fingerprint": "79ea139a-7e5c-4b78-abd7-bb8bbd02c700",
	"agent_role":        "Researcher",
	"crew_fingerprint":  "6d2eb9fd-85aa-4af2-9f0e-1630c72b06e6",
	"crew_id":           "3d86fbf1-c04c-4a91-8478-610847238a84",
	"crew_key":          "e0e467a06ac5abbbbfed25ac101c0909",
	"task_fingerprint":  "40a6eafc-7102-4ca0-87ee-af18c859e03b",
	"task_id":           "d76fde56-6272-4198-83fc-bd3034a86f75",
	"task_key":          "ad4694b4e962efb4262cf34539302ab3",
}

var toolWithParametersSpan = map[string]any{
	"openinference.span.kind": "TOOL",
	"tool.name":               "get_weather",
	"tool.description":        "Get weather for a city",
	"tool.parameters":         `{"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]}`,
	"input.value":             `{"city": "Seattle"}`,
	"output.value":            "72F and sunny in Seattle",
}

var attributesToRemoveSpan = map[string]any{
	"openinference.span.kind":                      "LLM",
	"llm.model_name":                               "gpt-4",
	"input.mime_type":                              "application/json",
	"output.mime_type":                             "application/json",
	"metadata":                                     `{"key":"value"}`,
	"llm.token_count.total":                        int64(100),
	"llm.system":                                   "openai",
	"llm.prompt_template.template":                 "Hello {name}",
	"llm.prompt_template.variables":                `{"name":"world"}`,
	"tool.parameters":                              `{"type":"object"}`,
	"llm.cost.total":                               "0.005",
	"llm.cost.prompt":                              "0.003",
	"llm.cost.completion":                          "0.002",
	"llm.function_call":                            `{"name":"search"}`,
	"llm.prompts":                                  "Tell me a joke",
	"llm.token_count.prompt_details.cache_read":    int64(50),
	"llm.token_count.prompt_details.cache_write":   int64(10),
	"llm.token_count.prompt_details.audio":         int64(5),
	"llm.token_count.completion_details.reasoning": int64(20),
	"llm.token_count.completion_details.audio":     int64(3),
}

var multimodalContentSpan = map[string]any{
	"openinference.span.kind":                                                 "LLM",
	"llm.input_messages.0.message.role":                                       "user",
	"llm.input_messages.0.message.contents.0.message_content.type":            "text",
	"llm.input_messages.0.message.contents.0.message_content.text":            "What's in this image?",
	"llm.input_messages.0.message.contents.1.message_content.type":            "image",
	"llm.input_messages.0.message.contents.1.message_content.image.image.url": "https://example.com/cat.jpg",
	"llm.output_messages.0.message.role":                                      "assistant",
	"llm.output_messages.0.message.content":                                   "I see a cat.",
}
