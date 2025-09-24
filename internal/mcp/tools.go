package mcp

// GetMCPTools 获取所有MCP工具定义
func GetMCPTools() []MCPTool {
	return []MCPTool{
		// 认证相关
		{
			Name:        "check_login_status",
			Description: "检查B站登录状态",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"account_name": map[string]interface{}{
						"type":        "string",
						"description": "账号名称（可选，默认使用当前账号）",
					},
				},
			},
		},
		{
			Name:        "list_accounts",
			Description: "列出所有已登录的账号",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "switch_account",
			Description: "切换当前使用的账号",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"account_name": map[string]interface{}{
						"type":        "string",
						"description": "要切换到的账号名称",
					},
				},
				"required": []string{"account_name"},
			},
		},

		// 评论相关
		{
			Name:        "post_comment",
			Description: "发表文字评论到视频",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_id": map[string]interface{}{
						"type":        "string",
						"description": "视频BV号或AV号（如：BV1234567890 或 av123456）",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "评论内容",
					},
					"account_name": map[string]interface{}{
						"type":        "string",
						"description": "指定使用的账号名称（可选，默认使用当前账号）",
					},
				},
				"required": []string{"video_id", "content"},
			},
		},
		{
			Name:        "post_image_comment",
			Description: "发表图片评论到视频",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_id": map[string]interface{}{
						"type":        "string",
						"description": "视频BV号或AV号",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "评论文字内容",
					},
					"image_path": map[string]interface{}{
						"type":        "string",
						"description": "本地图片文件路径",
					},
					"account_name": map[string]interface{}{
						"type":        "string",
						"description": "指定使用的账号名称（可选）",
					},
				},
				"required": []string{"video_id", "content", "image_path"},
			},
		},
		{
			Name:        "reply_comment",
			Description: "回复评论",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_id": map[string]interface{}{
						"type":        "string",
						"description": "视频BV号或AV号",
					},
					"parent_comment_id": map[string]interface{}{
						"type":        "string",
						"description": "父评论ID",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "回复内容",
					},
					"account_name": map[string]interface{}{
						"type":        "string",
						"description": "指定使用的账号名称（可选）",
					},
				},
				"required": []string{"video_id", "parent_comment_id", "content"},
			},
		},

		// 视频操作
		{
			Name:        "get_video_info",
			Description: "获取视频详细信息",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_id": map[string]interface{}{
						"type":        "string",
						"description": "视频BV号或AV号",
					},
				},
				"required": []string{"video_id"},
			},
		},
		{
			Name:        "like_video",
			Description: "点赞视频",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_id": map[string]interface{}{
						"type":        "string",
						"description": "视频BV号或AV号",
					},
					"account_name": map[string]interface{}{
						"type":        "string",
						"description": "指定使用的账号名称（可选）",
					},
				},
				"required": []string{"video_id"},
			},
		},
		{
			Name:        "coin_video",
			Description: "投币视频",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_id": map[string]interface{}{
						"type":        "string",
						"description": "视频BV号或AV号",
					},
					"coin_count": map[string]interface{}{
						"type":        "integer",
						"description": "投币数量（1或2）",
						"minimum":     1,
						"maximum":     2,
						"default":     1,
					},
					"account_name": map[string]interface{}{
						"type":        "string",
						"description": "指定使用的账号名称（可选）",
					},
				},
				"required": []string{"video_id"},
			},
		},
		{
			Name:        "favorite_video",
			Description: "收藏视频",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_id": map[string]interface{}{
						"type":        "string",
						"description": "视频BV号或AV号",
					},
					"folder_id": map[string]interface{}{
						"type":        "string",
						"description": "收藏夹ID（可选，默认收藏夹）",
					},
					"account_name": map[string]interface{}{
						"type":        "string",
						"description": "指定使用的账号名称（可选）",
					},
				},
				"required": []string{"video_id"},
			},
		},
		{
			Name:        "download_media",
			Description: "下载B站视频的媒体文件，支持音频、视频或合并文件下载，支持多种清晰度选择",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_id": map[string]interface{}{
						"type":        "string",
						"description": "视频BV号或AV号",
					},
					"media_type": map[string]interface{}{
						"type":        "string",
						"description": "媒体类型：audio=仅音频, video=仅视频, merged=音视频合并（默认）",
						"enum":        []string{"audio", "video", "merged"},
					},
					"quality": map[string]interface{}{
						"type":        "number",
						"description": "视频清晰度（可选）：16=360P, 32=480P, 64=720P, 80=1080P, 112=1080P+, 116=1080P60, 120=4K, 125=HDR, 127=8K。0=自动选择最佳",
					},
					"cid": map[string]interface{}{
						"type":        "number",
						"description": "视频分P的CID（可选，不指定则使用第一个分P）",
					},
					"output_dir": map[string]interface{}{
						"type":        "string",
						"description": "输出目录路径（可选，默认为./downloads）",
					},
					"account_name": map[string]interface{}{
						"type":        "string",
						"description": "指定使用的账号名称（可选，登录后可获取更高清晰度）",
					},
				},
				"required": []string{"video_id"},
			},
		},

		// 用户操作
		{
			Name:        "follow_user",
			Description: "关注用户",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"user_id": map[string]interface{}{
						"type":        "string",
						"description": "用户UID",
					},
					"account_name": map[string]interface{}{
						"type":        "string",
						"description": "指定使用的账号名称（可选）",
					},
				},
				"required": []string{"user_id"},
			},
		},
		{
			Name:        "get_user_videos",
			Description: "获取用户发布的视频列表",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"user_id": map[string]interface{}{
						"type":        "string",
						"description": "用户UID",
					},
					"page": map[string]interface{}{
						"type":        "integer",
						"description": "页码",
						"default":     1,
						"minimum":     1,
					},
					"page_size": map[string]interface{}{
						"type":        "integer",
						"description": "每页数量",
						"default":     20,
						"minimum":     1,
						"maximum":     50,
					},
				},
				"required": []string{"user_id"},
			},
		},

		// 可选功能
		{
			Name:        "transcribe_video",
			Description: "提取视频音频并转录为文字（需要安装Whisper）",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_id": map[string]interface{}{
						"type":        "string",
						"description": "视频BV号或AV号",
					},
					"language": map[string]interface{}{
						"type":        "string",
						"description": "语言代码（zh, en等）",
						"default":     "zh",
					},
				},
				"required": []string{"video_id"},
			},
		},

		// 视频流相关
		{
			Name:        "get_video_stream",
			Description: "获取视频流地址，支持MP4和DASH格式。使用步骤：1)先调用get_video_info获取CID 2)用获取的CID调用此工具",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_id": map[string]interface{}{
						"type":        "string",
						"description": "视频ID（BV号或av号）",
					},
					"cid": map[string]interface{}{
						"type":        "number",
						"description": "视频分P的CID，必须先用get_video_info获取。每个分P都有唯一CID，不能为0",
					},
					"quality": map[string]interface{}{
						"type":        "number",
						"description": "视频清晰度（可选）：16=360P, 32=480P, 64=720P, 80=1080P, 112=1080P+, 116=1080P60, 120=4K, 125=HDR, 127=8K",
					},
					"fnval": map[string]interface{}{
						"type":        "number",
						"description": "视频流格式（可选）：1=MP4, 16=DASH, 64=HDR, 128=4K, 256=杜比音频, 512=杜比视界, 1024=8K, 2048=AV1, 4048=所有DASH",
					},
					"platform": map[string]interface{}{
						"type":        "string",
						"description": "平台标识（可选）：pc=PC端（有防盗链），html5=移动端（无防盗链）",
					},
					"account_name": map[string]interface{}{
						"type":        "string",
						"description": "指定使用的账号名称（可选，登录后可获取更高清晰度）",
					},
				},
				"required": []string{"video_id", "cid"},
			},
		},
	}
}
