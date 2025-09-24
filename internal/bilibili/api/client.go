package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Client B站API客户端
type Client struct {
	httpClient *http.Client
	cookies    map[string]string
}

// NewClient 创建API客户端
func NewClient(cookies map[string]string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // 增加到60秒，支持较慢的API请求
		},
		cookies: cookies,
	}
}

// getHeaders 获取标准请求头
func (c *Client) getHeaders(referer string) map[string]string {
	return map[string]string{
		"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Referer":    referer,
		"Origin":     "https://www.bilibili.com",
		"Accept":     "application/json, text/plain, */*",
	}
}

// getCookieString 获取cookie字符串
func (c *Client) getCookieString() string {
	var parts []string
	for name, value := range c.cookies {
		parts = append(parts, fmt.Sprintf("%s=%s", name, value))
	}
	return strings.Join(parts, "; ")
}

// makeRequest 发起HTTP请求
func (c *Client) makeRequest(method, url string, data url.Values, headers map[string]string) ([]byte, error) {
	var req *http.Request
	var err error

	if method == "POST" {
		req, err = http.NewRequest(method, url, strings.NewReader(data.Encode()))
		if err != nil {
			return nil, errors.Wrap(err, "创建POST请求失败")
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	} else {
		if len(data) > 0 {
			url = url + "?" + data.Encode()
		}
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return nil, errors.Wrap(err, "创建GET请求失败")
		}
	}

	// 设置cookie
	req.Header.Set("Cookie", c.getCookieString())

	// 设置其他请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP请求失败")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "读取响应失败")
	}

	return body, nil
}

// NavResponse 导航API响应
type NavResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		IsLogin bool   `json:"isLogin"`
		Uname   string `json:"uname"`
		Mid     int64  `json:"mid"`
		Face    string `json:"face"`
	} `json:"data"`
}

// GetNavInfo 获取导航信息（用于验证登录状态和获取用户信息）
func (c *Client) GetNavInfo() (*NavResponse, error) {
	headers := c.getHeaders("https://www.bilibili.com")
	body, err := c.makeRequest("GET", "https://api.bilibili.com/x/web-interface/nav", nil, headers)
	if err != nil {
		return nil, err
	}

	var resp NavResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errors.Wrap(err, "解析导航API响应失败")
	}

	return &resp, nil
}

// CommentResponse 评论API响应
type CommentResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Rpid int64 `json:"rpid"` // 评论ID
	} `json:"data"`
}

// PostComment 发表评论
func (c *Client) PostComment(videoID, content string) (*CommentResponse, error) {
	// 从videoID获取aid
	aid, err := c.getVideoAid(videoID)
	if err != nil {
		return nil, errors.Wrap(err, "获取视频aid失败")
	}

	// 获取CSRF token
	csrf, exists := c.cookies["bili_jct"]
	if !exists {
		return nil, errors.New("缺少CSRF token (bili_jct)")
	}

	// 构建请求参数
	data := url.Values{
		"oid":     {strconv.FormatInt(aid, 10)},
		"type":    {"1"}, // 视频评论区
		"message": {content},
		"plat":    {"1"}, // web端
		"csrf":    {csrf},
	}

	headers := c.getHeaders(fmt.Sprintf("https://www.bilibili.com/video/%s", videoID))
	body, err := c.makeRequest("POST", "https://api.bilibili.com/x/v2/reply/add", data, headers)
	if err != nil {
		return nil, err
	}

	var resp CommentResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errors.Wrap(err, "解析评论API响应失败")
	}

	return &resp, nil
}

// VideoInfoResponse 视频信息API响应 - 完整版本
type VideoInfoResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Aid       int64  `json:"aid"`       // 视频AV号
		Bvid      string `json:"bvid"`      // 视频BV号
		Title     string `json:"title"`     // 视频标题
		Desc      string `json:"desc"`      // 视频简介
		Duration  int    `json:"duration"`  // 视频时长(秒)
		Cid       int64  `json:"cid"`       // 视频CID
		Pubdate   int64  `json:"pubdate"`   // 发布时间(Unix时间戳)
		Ctime     int64  `json:"ctime"`     // 上传时间(Unix时间戳)
		Pic       string `json:"pic"`       // 封面图片URL
		Tname     string `json:"tname"`     // 分区名称
		Copyright int    `json:"copyright"` // 1:原创 2:转载
		Videos    int    `json:"videos"`    // 分P数量
		State     int    `json:"state"`     // 视频状态
		Attribute int    `json:"attribute"` // 视频属性
		Owner     struct {
			Mid  int64  `json:"mid"`  // UP主UID
			Name string `json:"name"` // UP主昵称
			Face string `json:"face"` // UP主头像
		} `json:"owner"`
		Stat struct {
			View     int64 `json:"view"`     // 播放量
			Danmaku  int64 `json:"danmaku"`  // 弹幕数
			Reply    int64 `json:"reply"`    // 评论数
			Favorite int64 `json:"favorite"` // 收藏数
			Coin     int64 `json:"coin"`     // 投币数
			Share    int64 `json:"share"`    // 分享数
			Like     int64 `json:"like"`     // 点赞数
		} `json:"stat"`
		Rights struct {
			BP            int `json:"bp"`             // 是否允许承包
			Elec          int `json:"elec"`           // 是否支持充电
			Download      int `json:"download"`       // 是否允许下载
			Movie         int `json:"movie"`          // 是否为电影
			Pay           int `json:"pay"`            // 是否付费
			HD5           int `json:"hd5"`            // 是否有高码率
			NoReprint     int `json:"no_reprint"`     // 是否禁止转载
			Autoplay      int `json:"autoplay"`       // 是否自动播放
			UGCPay        int `json:"ugc_pay"`        // 是否UGC付费
			IsCooperation int `json:"is_cooperation"` // 是否联合投稿
		} `json:"rights"`
		Dimension struct {
			Width  int `json:"width"`  // 视频宽度
			Height int `json:"height"` // 视频高度
			Rotate int `json:"rotate"` // 是否旋转
		} `json:"dimension"`
		Pages []struct {
			Cid       int64  `json:"cid"`      // 分P的CID
			Page      int    `json:"page"`     // 分P序号
			From      string `json:"from"`     // 来源
			Part      string `json:"part"`     // 分P标题
			Duration  int    `json:"duration"` // 分P时长
			Vid       string `json:"vid"`      // 视频ID
			Weblink   string `json:"weblink"`  // 网页链接
			Dimension struct {
				Width  int `json:"width"`
				Height int `json:"height"`
				Rotate int `json:"rotate"`
			} `json:"dimension"`
		} `json:"pages"`
		Subtitle struct {
			AllowSubmit bool `json:"allow_submit"` // 是否允许提交字幕
			List        []struct {
				ID          int64  `json:"id"`           // 字幕ID
				Lan         string `json:"lan"`          // 语言代码
				LanDoc      string `json:"lan_doc"`      // 语言名称
				IsLock      bool   `json:"is_lock"`      // 是否锁定
				SubtitleURL string `json:"subtitle_url"` // 字幕文件URL
			} `json:"list"`
		} `json:"subtitle"`
		Staff []struct {
			Mid   int64  `json:"mid"`   // 成员UID
			Title string `json:"title"` // 成员角色
			Name  string `json:"name"`  // 成员昵称
			Face  string `json:"face"`  // 成员头像
		} `json:"staff"` // 合作成员信息
		Tags []struct {
			TagID   int64  `json:"tag_id"`   // 标签ID
			TagName string `json:"tag_name"` // 标签名称
		} `json:"tag"` // 视频标签列表
	} `json:"data"`
}

// videoIDToAID 辅助函数：将BV号或AV号转换为AID
func (c *Client) videoIDToAID(videoID string) (int64, error) {
	if strings.HasPrefix(videoID, "BV") {
		// 调用API转换BV到AID
		apiURL := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", videoID)
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return 0, errors.Wrap(err, "创建请求失败")
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Referer", "https://www.bilibili.com")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return 0, errors.Wrap(err, "API请求失败")
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return 0, errors.Wrap(err, "读取响应失败")
		}

		var viewResp struct {
			Code int `json:"code"`
			Data struct {
				AID int64 `json:"aid"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &viewResp); err != nil {
			return 0, errors.Wrap(err, "解析BV转AID API响应失败")
		}
		if viewResp.Code != 0 {
			return 0, errors.Errorf("BV转AID API返回错误: code %d", viewResp.Code)
		}
		return viewResp.Data.AID, nil
	} else if strings.HasPrefix(videoID, "av") || strings.HasPrefix(videoID, "AV") {
		aidStr := strings.TrimPrefix(strings.ToLower(videoID), "av")
		aid, err := strconv.ParseInt(aidStr, 10, 64)
		if err != nil {
			return 0, errors.New("无效的AV号格式")
		}
		return aid, nil
	}
	return 0, errors.New("无效的视频ID格式，应为BV号或AV号")
}

// getVideoAid 从videoID获取aid (已废弃，使用videoIDToAID)
func (c *Client) getVideoAid(videoID string) (int64, error) {
	headers := c.getHeaders(fmt.Sprintf("https://www.bilibili.com/video/%s", videoID))
	data := url.Values{
		"bvid": {videoID},
	}

	body, err := c.makeRequest("GET", "https://api.bilibili.com/x/web-interface/view", data, headers)
	if err != nil {
		return 0, err
	}

	var resp VideoInfoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, errors.Wrap(err, "解析视频信息API响应失败")
	}

	if resp.Code != 0 {
		return 0, errors.Errorf("获取视频信息失败: %s", resp.Message)
	}

	return resp.Data.Aid, nil
}

// GetVideoInfo 获取视频信息
func (c *Client) GetVideoInfo(videoID string) (*VideoInfoResponse, error) {
	headers := c.getHeaders(fmt.Sprintf("https://www.bilibili.com/video/%s", videoID))
	data := url.Values{
		"bvid": {videoID},
	}

	body, err := c.makeRequest("GET", "https://api.bilibili.com/x/web-interface/view", data, headers)
	if err != nil {
		return nil, err
	}

	var resp VideoInfoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errors.Wrap(err, "解析视频信息API响应失败")
	}

	return &resp, nil
}

// LikeResponse 点赞API响应
type LikeResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// LikeVideo 点赞视频
func (c *Client) LikeVideo(videoID string, like int) (*LikeResponse, error) {
	// 获取CSRF token
	csrf, exists := c.cookies["bili_jct"]
	if !exists || csrf == "" {
		return nil, errors.New("缺少CSRF token (bili_jct)")
	}

	// 根据bilibili-API-collect的规范，优先使用aid，如果videoID是BV号则需要转换
	var data url.Values
	if strings.HasPrefix(videoID, "BV") {
		// 使用BVID
		data = url.Values{
			"bvid": {videoID},
			"like": {strconv.Itoa(like)},
			"csrf": {csrf},
		}
	} else {
		// 假设是AID
		data = url.Values{
			"aid":  {videoID},
			"like": {strconv.Itoa(like)},
			"csrf": {csrf},
		}
	}

	headers := c.getHeaders(fmt.Sprintf("https://www.bilibili.com/video/%s", videoID))
	// 确保Content-Type正确
	headers["Content-Type"] = "application/x-www-form-urlencoded; charset=UTF-8"

	body, err := c.makeRequest("POST", "https://api.bilibili.com/x/web-interface/archive/like", data, headers)
	if err != nil {
		return nil, err
	}

	var resp LikeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errors.Wrap(err, "解析点赞API响应失败")
	}

	return &resp, nil
}

// PlayUrlResponse 视频播放地址API响应
type PlayUrlResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Dash struct {
			Duration int `json:"duration"` // 视频总时长
			Audio    []struct {
				ID        int    `json:"id"`        // 音频流ID
				BaseURL   string `json:"baseUrl"`   // 音频流地址
				Bandwidth int    `json:"bandwidth"` // 带宽
				MimeType  string `json:"mimeType"`  // MIME类型
				Codecs    string `json:"codecs"`    // 编码格式
			} `json:"audio"`
			Video []struct {
				ID        int    `json:"id"`        // 视频流ID
				BaseURL   string `json:"baseUrl"`   // 视频流地址
				Bandwidth int    `json:"bandwidth"` // 带宽
				MimeType  string `json:"mimeType"`  // MIME类型
				Codecs    string `json:"codecs"`    // 编码格式
				Width     int    `json:"width"`     // 宽度
				Height    int    `json:"height"`    // 高度
			} `json:"video"`
		} `json:"dash"`
	} `json:"data"`
}

// GetPlayUrl 获取视频播放地址
func (c *Client) GetPlayUrl(videoID string) (*PlayUrlResponse, error) {
	// 首先获取视频信息以获取CID
	videoInfo, err := c.GetVideoInfo(videoID)
	if err != nil {
		return nil, errors.Wrap(err, "获取视频信息失败")
	}

	if videoInfo.Code != 0 {
		return nil, errors.Errorf("获取视频信息失败: %s (code: %d)", videoInfo.Message, videoInfo.Code)
	}

	// 使用第一个分P的CID
	cid := videoInfo.Data.Pages[0].Cid

	// 构建播放地址API请求
	params := url.Values{
		"fnval": {"16"}, // DASH格式
		"fnver": {"0"},
		"fourk": {"1"},
	}

	// 添加视频ID参数
	if strings.HasPrefix(videoID, "BV") {
		params.Set("bvid", videoID)
	} else if strings.HasPrefix(videoID, "av") || strings.HasPrefix(videoID, "AV") {
		aidStr := strings.TrimPrefix(strings.ToLower(videoID), "av")
		params.Set("avid", aidStr)
	} else {
		return nil, errors.New("无效的视频ID格式")
	}

	params.Set("cid", fmt.Sprintf("%d", cid))

	apiURL := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?%s", params.Encode())
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "创建请求失败")
	}

	// 设置请求头
	headers := c.getHeaders(fmt.Sprintf("https://www.bilibili.com/video/%s", videoID))
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 添加cookies
	if cookieStr := c.getCookieString(); cookieStr != "" {
		req.Header.Set("Cookie", cookieStr)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "API请求失败")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "读取响应失败")
	}

	var playUrlResp PlayUrlResponse
	if err := json.Unmarshal(body, &playUrlResp); err != nil {
		return nil, errors.Wrap(err, "解析API响应失败")
	}

	return &playUrlResp, nil
}

// UserVideosResponse 用户视频列表API响应
type UserVideosResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List struct {
			Tlist map[string]struct {
				Tid   int    `json:"tid"`   // 分区ID
				Count int    `json:"count"` // 该分区视频数量
				Name  string `json:"name"`  // 分区名称
			} `json:"tlist"` // 分区统计
			Vlist []struct {
				Aid         int64  `json:"aid"`          // 视频AV号
				Bvid        string `json:"bvid"`         // 视频BV号
				Title       string `json:"title"`        // 视频标题
				Subtitle    string `json:"subtitle"`     // 视频副标题
				Description string `json:"description"`  // 视频简介
				Pic         string `json:"pic"`          // 视频封面
				Play        int64  `json:"play"`         // 播放量
				VideoReview int64  `json:"video_review"` // 弹幕数
				Comment     int64  `json:"comment"`      // 评论数
				Length      string `json:"length"`       // 视频时长
				Created     int64  `json:"created"`      // 发布时间戳
				Mid         int64  `json:"mid"`          // UP主UID
				Author      string `json:"author"`       // UP主昵称
				Typeid      int    `json:"typeid"`       // 分区ID
				Typename    string `json:"typename"`     // 分区名称
			} `json:"vlist"` // 视频列表
		} `json:"list"`
		Page struct {
			Pn    int `json:"pn"`    // 当前页码
			Ps    int `json:"ps"`    // 每页数量
			Count int `json:"count"` // 总数量
		} `json:"page"` // 分页信息
	} `json:"data"`
}

// GetUserVideos 获取用户投稿视频列表
func (c *Client) GetUserVideos(userID string, page, pageSize int) (*UserVideosResponse, error) {
	// 构建API请求参数 - 使用更稳定的参数组合
	params := url.Values{
		"mid":   {userID},
		"pn":    {fmt.Sprintf("%d", page)},
		"ps":    {fmt.Sprintf("%d", pageSize)},
		"order": {"pubdate"}, // 按发布时间排序
		"jsonp": {"jsonp"},   // 添加jsonp参数提高兼容性
	}

	apiURL := fmt.Sprintf("https://api.bilibili.com/x/space/arc/search?%s", params.Encode())
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "创建请求失败")
	}

	// 设置请求头 - 模拟真实浏览器访问
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://space.bilibili.com/"+userID)
	req.Header.Set("Origin", "https://space.bilibili.com")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")

	// 添加cookies（如果有的话）
	if cookieStr := c.getCookieString(); cookieStr != "" {
		req.Header.Set("Cookie", cookieStr)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "API请求失败")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "读取响应失败")
	}

	var userVideosResp UserVideosResponse
	if err := json.Unmarshal(body, &userVideosResp); err != nil {
		return nil, errors.Wrap(err, "解析API响应失败")
	}

	return &userVideosResp, nil
}

// CoinVideoResponse 投币视频API响应
type CoinVideoResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Like bool `json:"like"` // 是否同时点赞
	} `json:"data"`
}

// CoinVideo 投币视频
func (c *Client) CoinVideo(videoID string, coinCount int, alsoLike bool) (*CoinVideoResponse, error) {
	// 转换videoID为AID
	aid, err := c.videoIDToAID(videoID)
	if err != nil {
		return nil, errors.Wrap(err, "转换视频ID为AID失败")
	}

	// 获取CSRF token
	csrf, ok := c.cookies["bili_jct"]
	if !ok || csrf == "" {
		return nil, errors.New("缺少CSRF token，请确保已登录")
	}

	data := url.Values{
		"aid":          {fmt.Sprintf("%d", aid)},
		"multiply":     {fmt.Sprintf("%d", coinCount)},
		"select_like":  {"0"},
		"cross_domain": {"true"},
		"csrf":         {csrf},
	}

	if alsoLike {
		data.Set("select_like", "1")
	}

	headers := c.getHeaders(fmt.Sprintf("https://www.bilibili.com/video/%s", videoID))
	body, err := c.makeRequest("POST", "https://api.bilibili.com/x/web-interface/coin/add", data, headers)
	if err != nil {
		return nil, err
	}

	var resp CoinVideoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errors.Wrap(err, "解析投币API响应失败")
	}

	return &resp, nil
}

// FavoriteVideoResponse 收藏视频API响应
type FavoriteVideoResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Prompt bool `json:"prompt"` // 是否为未关注用户收藏
	} `json:"data"`
}

// FavoriteVideo 收藏视频
func (c *Client) FavoriteVideo(videoID string, folderIDs []string, addMedia bool) (*FavoriteVideoResponse, error) {
	// 转换videoID为AID
	aid, err := c.videoIDToAID(videoID)
	if err != nil {
		return nil, errors.Wrap(err, "转换视频ID为AID失败")
	}

	// 获取CSRF token
	csrf, ok := c.cookies["bili_jct"]
	if !ok || csrf == "" {
		return nil, errors.New("缺少CSRF token，请确保已登录")
	}

	// 如果没有指定收藏夹，尝试获取用户默认收藏夹
	if len(folderIDs) == 0 {
		// 先尝试获取用户的收藏夹列表来找到默认收藏夹
		defaultFolders, err := c.getDefaultFavoriteFolder()
		if err != nil {
			// 如果获取失败，使用通用的默认值
			folderIDs = []string{"1"}
		} else {
			folderIDs = defaultFolders
		}
	}

	data := url.Values{
		"rid":           {fmt.Sprintf("%d", aid)},
		"type":          {"2"}, // 视频类型
		"add_media_ids": {strings.Join(folderIDs, ",")},
		"del_media_ids": {""},
		"csrf":          {csrf},
	}

	headers := c.getHeaders(fmt.Sprintf("https://www.bilibili.com/video/%s", videoID))
	// 设置正确的 Referer
	headers["Referer"] = fmt.Sprintf("https://www.bilibili.com/video/%s", videoID)

	body, err := c.makeRequest("POST", "https://api.bilibili.com/x/v3/fav/resource/deal", data, headers)
	if err != nil {
		return nil, err
	}

	var resp FavoriteVideoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errors.Wrap(err, "解析收藏API响应失败")
	}

	return &resp, nil
}

// getDefaultFavoriteFolder 获取用户的默认收藏夹ID
func (c *Client) getDefaultFavoriteFolder() ([]string, error) {
	// 尝试获取用户的收藏夹列表
	headers := c.getHeaders("https://www.bilibili.com")

	// 构建获取收藏夹列表的URL
	// 需要用户的mid，但我们这里先尝试不指定mid的方式
	apiURL := "https://api.bilibili.com/x/v3/fav/folder/created/list-all?up_mid=0"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return []string{"1"}, nil // 返回默认值
	}

	// 设置headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return []string{"1"}, nil // 返回默认值
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []string{"1"}, nil // 返回默认值
	}

	var favResp struct {
		Code int `json:"code"`
		Data struct {
			List []struct {
				ID    int64  `json:"id"`
				Title string `json:"title"`
				Attr  int    `json:"attr"`
			} `json:"list"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &favResp); err != nil || favResp.Code != 0 {
		return []string{"1"}, nil // 返回默认值
	}

	// 寻找默认收藏夹（通常是第一个或者title为"默认收藏夹"的）
	if len(favResp.Data.List) > 0 {
		defaultFolder := favResp.Data.List[0]
		return []string{fmt.Sprintf("%d", defaultFolder.ID)}, nil
	}

	return []string{"1"}, nil // 返回默认值
}

// FollowUserResponse 关注用户API响应
type FollowUserResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Status int `json:"status"` // 关注状态
	} `json:"data"`
}

// VideoStreamResponse 视频流API响应
type VideoStreamResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	TTL     int              `json:"ttl"`
	Data    *VideoStreamData `json:"data"`
}

// VideoStreamData 视频流数据
type VideoStreamData struct {
	From              string          `json:"from"`
	Result            string          `json:"result"`
	Message           string          `json:"message"`
	Quality           int             `json:"quality"`
	Format            string          `json:"format"`
	TimeLength        int64           `json:"timelength"`
	AcceptFormat      string          `json:"accept_format"`
	AcceptDescription []string        `json:"accept_description"`
	AcceptQuality     []int           `json:"accept_quality"`
	VideoCodecID      int             `json:"video_codecid"`
	SeekParam         string          `json:"seek_param"`
	SeekType          string          `json:"seek_type"`
	DURL              []VideoSegment  `json:"durl,omitempty"` // MP4/FLV格式
	DASH              *DASHInfo       `json:"dash,omitempty"` // DASH格式
	SupportFormats    []SupportFormat `json:"support_formats"`
	HighFormat        interface{}     `json:"high_format"`
	LastPlayTime      int64           `json:"last_play_time"`
	LastPlayCID       int64           `json:"last_play_cid"`
}

// VideoSegment 视频分段信息（MP4/FLV格式）
type VideoSegment struct {
	Order     int      `json:"order"`
	Length    int64    `json:"length"`
	Size      int64    `json:"size"`
	Ahead     string   `json:"ahead"`
	VHead     string   `json:"vhead"`
	URL       string   `json:"url"`
	BackupURL []string `json:"backup_url"`
}

// DASHInfo DASH格式视频流信息
type DASHInfo struct {
	Duration      int          `json:"duration"`
	MinBufferTime float64      `json:"minBufferTime"`
	Video         []DASHStream `json:"video"`
	Audio         []DASHStream `json:"audio"`
	Dolby         *DolbyInfo   `json:"dolby,omitempty"`
	FLAC          *FLACInfo    `json:"flac,omitempty"`
}

// DASHStream DASH视频/音频流
type DASHStream struct {
	ID           int          `json:"id"`
	BaseURL      string       `json:"baseUrl"`
	BackupURL    []string     `json:"backupUrl"`
	Bandwidth    int64        `json:"bandwidth"`
	MimeType     string       `json:"mimeType"`
	Codecs       string       `json:"codecs"`
	Width        int          `json:"width,omitempty"`
	Height       int          `json:"height,omitempty"`
	FrameRate    string       `json:"frameRate,omitempty"`
	SAR          string       `json:"sar,omitempty"`
	StartWithSAP int          `json:"startWithSap,omitempty"`
	SegmentBase  *SegmentBase `json:"SegmentBase,omitempty"`
	CodecID      int          `json:"codecid"`
}

// SegmentBase 分段基础信息
type SegmentBase struct {
	Initialization string `json:"Initialization"`
	IndexRange     string `json:"indexRange"`
}

// DolbyInfo 杜比音效信息
type DolbyInfo struct {
	Type  int          `json:"type"`
	Audio []DASHStream `json:"audio"`
}

// FLACInfo 无损音轨信息
type FLACInfo struct {
	Display bool       `json:"display"`
	Audio   DASHStream `json:"audio"`
}

// SupportFormat 支持的格式信息
type SupportFormat struct {
	Quality        int      `json:"quality"`
	Format         string   `json:"format"`
	NewDescription string   `json:"new_description"`
	DisplayDesc    string   `json:"display_desc"`
	Superscript    string   `json:"superscript"`
	Codecs         []string `json:"codecs"`
}

// FollowUser 关注用户
func (c *Client) FollowUser(userID string, action int) (*FollowUserResponse, error) {
	// 获取CSRF token
	csrf, ok := c.cookies["bili_jct"]
	if !ok || csrf == "" {
		return nil, errors.New("缺少CSRF token，请确保已登录")
	}

	data := url.Values{
		"fid":    {userID},
		"act":    {fmt.Sprintf("%d", action)}, // 1:关注 2:取消关注
		"re_src": {"14"},
		"csrf":   {csrf},
	}

	headers := c.getHeaders("https://www.bilibili.com")
	body, err := c.makeRequest("POST", "https://api.bilibili.com/x/relation/modify", data, headers)
	if err != nil {
		return nil, err
	}

	var resp FollowUserResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errors.Wrap(err, "解析关注API响应失败")
	}

	return &resp, nil
}

// ReplyCommentResponse 回复评论API响应
type ReplyCommentResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		RPID        string `json:"rpid_str"`     // 回复ID
		Dialog      string `json:"dialog"`       // 对话ID
		Root        string `json:"root"`         // 根评论ID
		Parent      string `json:"parent"`       // 父评论ID
		NeedCaptcha bool   `json:"need_captcha"` // 是否需要验证码
	} `json:"data"`
}

// ReplyComment 回复评论
func (c *Client) ReplyComment(videoID, parentCommentID, content string) (*ReplyCommentResponse, error) {
	// 转换videoID为AID
	aid, err := c.videoIDToAID(videoID)
	if err != nil {
		return nil, errors.Wrap(err, "转换视频ID为AID失败")
	}

	// 获取CSRF token
	csrf, ok := c.cookies["bili_jct"]
	if !ok || csrf == "" {
		return nil, errors.New("缺少CSRF token，请确保已登录")
	}

	data := url.Values{
		"oid":     {fmt.Sprintf("%d", aid)},
		"type":    {"1"}, // 1代表视频评论区
		"root":    {parentCommentID},
		"parent":  {parentCommentID},
		"message": {content},
		"plat":    {"1"}, // 1代表web端
		"csrf":    {csrf},
	}

	headers := c.getHeaders(fmt.Sprintf("https://www.bilibili.com/video/%s", videoID))
	body, err := c.makeRequest("POST", "https://api.bilibili.com/x/v2/reply/add", data, headers)
	if err != nil {
		return nil, err
	}

	var resp ReplyCommentResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errors.Wrap(err, "解析回复评论API响应失败")
	}

	return &resp, nil
}

// GetVideoStream 获取视频流地址
func (c *Client) GetVideoStream(videoID string, cid int64, quality int, fnval int, platform string) (*VideoStreamResponse, error) {
	// 转换videoID为AID
	aid, err := c.videoIDToAID(videoID)
	if err != nil {
		return nil, errors.Wrap(err, "转换视频ID为AID失败")
	}

	// 构建请求参数
	params := url.Values{
		"avid":  {fmt.Sprintf("%d", aid)},
		"cid":   {fmt.Sprintf("%d", cid)},
		"fnval": {fmt.Sprintf("%d", fnval)},
		"fnver": {"0"},
		"fourk": {"1"},
		"otype": {"json"},
	}

	// 添加可选参数
	if quality > 0 {
		params.Set("qn", fmt.Sprintf("%d", quality))
	}
	if platform != "" {
		params.Set("platform", platform)
	}

	// 如果用户已登录，可以获取更高清晰度
	if _, hasSession := c.cookies["SESSDATA"]; hasSession {
		params.Set("try_look", "1")
	}

	// 构建请求URL
	apiURL := "https://api.bilibili.com/x/player/wbi/playurl?" + params.Encode()

	// 创建请求
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "创建请求失败")
	}

	// 设置headers
	headers := c.getHeaders(fmt.Sprintf("https://www.bilibili.com/video/%s", videoID))
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "发送请求失败")
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "读取响应失败")
	}

	// 解析响应
	var streamResp VideoStreamResponse
	if err := json.Unmarshal(body, &streamResp); err != nil {
		return nil, errors.Wrap(err, "解析视频流API响应失败")
	}

	// 检查API响应状态
	if streamResp.Code != 0 {
		return nil, fmt.Errorf("获取视频流失败: %s (code: %d)", streamResp.Message, streamResp.Code)
	}

	return &streamResp, nil
}
