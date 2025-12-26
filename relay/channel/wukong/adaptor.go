package wukong

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const ChannelName = "wukong"

var ModelList = []string{
	"kiro-claude-haiku-4-5-agentic",
	"kiro-claude-opus-4-5",
	"kiro-claude-sonnet-4-5",
	"kiro-claude-opus-4-5-agentic",
	"kiro-claude-sonnet-4-5-agentic",
}

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("wukong adaptor: relay info is nil")
	}
	// 根据请求路径判断使用哪个端点
	// /chat-stream 使用 wukong 原生格式
	// 其他（如测试）使用 OpenAI 格式的 /v1/chat/completions
	if strings.Contains(info.RequestURLPath, "chat-stream") {
		return fmt.Sprintf("%s/chat-stream", info.ChannelBaseUrl), nil
	}
	// 默认使用 OpenAI 兼容接口
	return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	if info == nil {
		return errors.New("wukong adaptor: relay info is nil")
	}
	if info.ApiKey == "" {
		return errors.New("wukong adaptor: api key is required")
	}
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	req.Set("Content-Type", "application/json")
	req.Set("Accept", "application/json")
	return nil
}

// WukongRequest wukong 请求格式
type WukongRequest struct {
	EncryptedData string   `json:"encrypted_data"`
	Data          string   `json:"data"`
	Images        []string `json:"images"`
	Model         string   `json:"model"`
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("wukong adaptor: request is nil")
	}
	
	// 如果不是 /chat-stream 请求，直接透传 OpenAI 格式
	if !strings.Contains(info.RequestURLPath, "chat-stream") {
		return request, nil
	}
	
	// /chat-stream 请求需要转换为 wukong 格式
	// 将 OpenAI Messages 转换为 wukong data 格式
	var dataBuilder strings.Builder
	for _, msg := range request.Messages {
		if content, ok := msg.Content.(string); ok {
			dataBuilder.WriteString(content)
			dataBuilder.WriteString("\n")
		}
	}
	
	wukongReq := WukongRequest{
		EncryptedData: "==",
		Data:          dataBuilder.String(),
		Images:        []string{},
		Model:         request.Model,
	}
	
	common.SysLog(fmt.Sprintf("[Wukong] Converted to WukongRequest: model=%s, data_length=%d", wukongReq.Model, len(wukongReq.Data)))
	return wukongReq, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	// 读取请求体
	bodyBytes, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, fmt.Errorf("wukong adaptor: failed to read request body: %w", err)
	}
	
	// 解析请求并计算输入 token
	var reqData map[string]interface{}
	if err := common.UnmarshalJsonStr(string(bodyBytes), &reqData); err == nil {
		// 从 data 字段获取明文内容计算 token
		if data, ok := reqData["data"].(string); ok && len(data) > 0 {
			model := ""
			if m, ok := reqData["model"].(string); ok {
				model = m
			}
			promptTokens := service.CountTextToken(data, model)
			info.SetEstimatePromptTokens(promptTokens)
			common.SysLog(fmt.Sprintf("[Wukong] data length: %d, estimated prompt tokens: %d", len(data), promptTokens))
		}
	}
	
	// 发送请求
	return channel.DoApiRequest(a, c, info, bytes.NewReader(bodyBytes))
}

// WukongStreamResponse 表示 Wukong 上游返回的流式响应格式
type WukongStreamResponse struct {
	Text       string `json:"text"`
	StopReason string `json:"stop_reason"`
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if resp == nil {
		return nil, types.NewError(errors.New("wukong adaptor: empty response"), types.ErrorCodeBadResponse)
	}

	defer resp.Body.Close()

	// wukong /chat-stream 端点总是返回流式响应
	if strings.Contains(info.RequestURLPath, "chat-stream") {
		info.IsStream = true
	}

	// 设置响应头
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Status(resp.StatusCode)

	// 读取并解析响应，同时透传给客户端
	var responseTextBuilder strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	// 增加 buffer 大小以处理长行
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// 记录首字时间
		info.SetFirstResponseTime()

		// 打印响应行用于调试
		common.SysLog(fmt.Sprintf("[Wukong] Response Line: %s", line))

		// 写入原始行到客户端
		c.Writer.Write([]byte(line + "\n"))
		c.Writer.Flush()

		// 解析 JSON 提取 text 字段用于 token 计算
		var streamResp WukongStreamResponse
		if err := common.UnmarshalJsonStr(line, &streamResp); err == nil {
			responseTextBuilder.WriteString(streamResp.Text)
		}
	}

	// 计算 token
	responseText := responseTextBuilder.String()
	promptTokens := info.GetEstimatePromptTokens()
	completionTokens := service.CountTextToken(responseText, info.UpstreamModelName)

	usage := &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}

	return usage, nil
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

// 以下方法不支持，返回错误

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("wukong adaptor: ConvertImageRequest is not supported")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("wukong adaptor: ConvertRerankRequest is not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("wukong adaptor: ConvertEmbeddingRequest is not supported")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("wukong adaptor: ConvertAudioRequest is not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("wukong adaptor: ConvertOpenAIResponsesRequest is not supported")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("wukong adaptor: ConvertClaudeRequest is not supported")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("wukong adaptor: ConvertGeminiRequest is not supported")
}
