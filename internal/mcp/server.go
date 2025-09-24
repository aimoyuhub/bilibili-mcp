package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/shirenchuang/bilibili-mcp/internal/bilibili/auth"
	"github.com/shirenchuang/bilibili-mcp/internal/browser"
	"github.com/shirenchuang/bilibili-mcp/pkg/config"
	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

// Server MCP服务器
type Server struct {
	config       *config.Config
	browserPool  *browser.BrowserPool
	loginService *auth.LoginService
}

// NewServer 创建MCP服务器
func NewServer(cfg *config.Config, browserPool *browser.BrowserPool) *Server {
	return &Server{
		config:       cfg,
		browserPool:  browserPool,
		loginService: auth.NewLoginService(),
	}
}

// ServeHTTP 处理HTTP请求
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Mcp-Session-Id")

	// 处理OPTIONS请求
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case "GET":
		s.handleSSEConnection(w, r)
	case "POST":
		s.handleJSONRPCRequest(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSSEConnection 处理SSE连接
func (s *Server) handleSSEConnection(w http.ResponseWriter, r *http.Request) {
	// 检查是否支持SSE
	if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		http.Error(w, "SSE not requested", http.StatusBadRequest)
		return
	}

	// 设置SSE响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 发送初始化消息
	fmt.Fprintf(w, "event: open\n")
	fmt.Fprintf(w, "data: {\"type\":\"connection\",\"status\":\"connected\"}\n\n")

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// 保持连接打开
	<-r.Context().Done()
}

// handleJSONRPCRequest 处理JSON-RPC请求
func (s *Server) handleJSONRPCRequest(w http.ResponseWriter, r *http.Request) {
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendError(w, nil, -32700, "Parse error", err)
		return
	}
	defer r.Body.Close()

	// 解析JSON-RPC请求
	var request JSONRPCRequest
	if err := json.Unmarshal(body, &request); err != nil {
		s.sendError(w, nil, -32700, "Parse error", err)
		return
	}

	logger.Infof("收到MCP请求: %s", request.Method)

	// 处理请求
	response := s.processRequest(&request, r.Context())

	// 发送响应
	s.sendJSONResponse(w, response)
}

// processRequest 处理请求
func (s *Server) processRequest(request *JSONRPCRequest, ctx context.Context) *JSONRPCResponse {
	switch request.Method {
	case "initialize":
		return s.handleInitialize(request)
	case "initialized":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  map[string]interface{}{},
			ID:      request.ID,
		}
	case "ping":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  map[string]interface{}{"status": "ok"},
			ID:      request.ID,
		}
	case "tools/list":
		return s.handleToolsList(request)
	case "tools/call":
		return s.handleToolCall(ctx, request)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
			ID: request.ID,
		}
	}
}

// handleInitialize 处理初始化请求
func (s *Server) handleInitialize(request *JSONRPCRequest) *JSONRPCResponse {
	result := InitializeResult{
		ProtocolVersion: "2025-03-26",
		Capabilities: map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "bilibili-mcp",
			Version: "1.0.0",
		},
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      request.ID,
	}
}

// handleToolsList 处理工具列表请求
func (s *Server) handleToolsList(request *JSONRPCRequest) *JSONRPCResponse {
	tools := GetMCPTools()

	result := ToolsListResult{
		Tools: tools,
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      request.ID,
	}
}

// handleToolCall 处理工具调用
func (s *Server) handleToolCall(ctx context.Context, request *JSONRPCRequest) *JSONRPCResponse {
	// 设置工具调用超时时间为5分钟（支持音频下载等耗时操作）
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	// 解析参数
	params, ok := request.Params.(map[string]interface{})
	if !ok {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
			ID: request.ID,
		}
	}

	toolName, _ := params["name"].(string)
	toolArgs, _ := params["arguments"].(map[string]interface{})

	logger.Infof("执行工具调用: %s", toolName)

	var result *MCPToolResult

	switch toolName {
	case "check_login_status":
		result = s.handleCheckLoginStatus(ctx, toolArgs)
	case "list_accounts":
		result = s.handleListAccounts(ctx, toolArgs)
	case "switch_account":
		result = s.handleSwitchAccount(ctx, toolArgs)
	case "post_comment":
		result = s.handlePostComment(ctx, toolArgs)
	case "post_image_comment":
		result = s.handlePostImageComment(ctx, toolArgs)
	case "reply_comment":
		result = s.handleReplyComment(ctx, toolArgs)
	case "get_video_info":
		result = s.handleGetVideoInfo(ctx, toolArgs)
	case "like_video":
		result = s.handleLikeVideo(ctx, toolArgs)
	case "download_media":
		result = s.handleDownloadMedia(ctx, toolArgs)
	case "coin_video":
		result = s.handleCoinVideo(ctx, toolArgs)
	case "favorite_video":
		result = s.handleFavoriteVideo(ctx, toolArgs)
	case "follow_user":
		result = s.handleFollowUser(ctx, toolArgs)
	case "get_user_videos":
		result = s.handleGetUserVideos(ctx, toolArgs)
	case "transcribe_video":
		result = s.handleTranscribeVideo(ctx, toolArgs)
	case "get_video_stream":
		result = s.handleGetVideoStream(ctx, toolArgs)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32602,
				Message: fmt.Sprintf("Unknown tool: %s", toolName),
			},
			ID: request.ID,
		}
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      request.ID,
	}
}

// sendJSONResponse 发送JSON响应
func (s *Server) sendJSONResponse(w http.ResponseWriter, response *JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Errorf("编码响应失败: %v", err)
	}
}

// sendError 发送错误响应
func (s *Server) sendError(w http.ResponseWriter, id interface{}, code int, message string, err error) {
	response := &JSONRPCResponse{
		JSONRPC: "2.0",
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
		ID: id,
	}

	if err != nil {
		response.Error.Data = err.Error()
		logger.Errorf("MCP错误: %s - %v", message, err)
	}

	s.sendJSONResponse(w, response)
}

// createToolResult 创建工具结果
func (s *Server) createToolResult(content string, isError bool) *MCPToolResult {
	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: content,
		}},
		IsError: isError,
	}
}

// createErrorResult 创建错误结果
func (s *Server) createErrorResult(err error) *MCPToolResult {
	return s.createToolResult(fmt.Sprintf("操作失败: %v", err), true)
}

// getAccountName 获取账号名称
func (s *Server) getAccountName(args map[string]interface{}) string {
	if accountName, ok := args["account_name"].(string); ok {
		return accountName
	}
	return "" // 空字符串表示使用默认账号
}

// validateVideoID 验证视频ID格式
func (s *Server) validateVideoID(videoID string) error {
	if videoID == "" {
		return errors.New("视频ID不能为空")
	}

	// 检查是否是BV号或AV号格式
	if !strings.HasPrefix(videoID, "BV") && !strings.HasPrefix(videoID, "av") {
		return errors.New("视频ID格式错误，应为BV号（如BV1234567890）或AV号（如av123456）")
	}

	return nil
}
