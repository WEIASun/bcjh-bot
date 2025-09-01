package messageservice

import (
	"bcjh-bot/dao"
	"bcjh-bot/scheduler"
	"bcjh-bot/util"
	"bcjh-bot/util/e"
	"bcjh-bot/util/logger"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	TopicImageSaveDir       = "./images/topics"
	TopicImageServerBaseURL = "http://localhost:8080" // 静态文件服务地址
)

func init() {
	if err := os.MkdirAll(TopicImageSaveDir, os.ModePerm); err != nil {
		logger.Errorf("创建主题图片目录失败: %v", err)
	}
}

func TopicQuery(c *scheduler.Context) {
	arg := c.PretreatedMessage

	if util.HasPrefixIn(arg, "新增", "添加") {
		if !dao.IsSuperAdmin(c.GetSenderId()) {
			_, _ = c.Reply(e.PermissionDeniedNote)
			return
		}

		params := strings.Split(arg, "*")
		if len(params) < 3 {
			_, _ = c.Reply("参数格式错误! 正确格式: 新增*关键词*内容")
			return
		}
		keyword := strings.TrimSpace(params[1])
		value := strings.TrimSpace(params[2])

		modifiedValue, imagePaths, err := processTopicImagesInContent(value, keyword)
		if err != nil {
			_, _ = c.Reply(fmt.Sprintf("图片处理失败: %v", err))
			return
		}

		if err := dao.CreateTopic(keyword, modifiedValue, imagePaths); err != nil {
			_, _ = c.Reply(err.Error())
		} else {
			msg := fmt.Sprintf("✅ 主题「%s」添加成功！\n📝 内容：%s", keyword, modifiedValue)
			if imagePaths != "" {
				msg += fmt.Sprintf("\n🖼 本地图片路径：%s", strings.ReplaceAll(imagePaths, ";", "\n"))
			}
			_, _ = c.Reply(msg)
		}
		return
	}

	if util.HasPrefixIn(arg, "更新", "修改") {
		if !dao.IsSuperAdmin(c.GetSenderId()) {
			_, _ = c.Reply(e.PermissionDeniedNote)
			return
		}

		params := strings.Split(arg, "*")
		if len(params) < 3 {
			_, _ = c.Reply("参数格式错误! 正确格式: 更新*关键词*新内容")
			return
		}
		keyword := strings.TrimSpace(params[1])
		value := strings.TrimSpace(params[2])

		modifiedValue, imagePaths, err := processTopicImagesInContent(value, keyword)
		if err != nil {
			_, _ = c.Reply(fmt.Sprintf("图片处理失败: %v", err))
			return
		}

		if err := dao.UpdateTopic(keyword, modifiedValue, imagePaths); err != nil {
			_, _ = c.Reply(err.Error())
		} else {
			msg := fmt.Sprintf("🔄 主题「%s」更新成功！\n📝 新内容：%s", keyword, modifiedValue)
			if imagePaths != "" {
				msg += fmt.Sprintf("\n🖼 本地图片路径：%s", strings.ReplaceAll(imagePaths, ";", "\n"))
			}
			_, _ = c.Reply(msg)
		}
		return
	}

	if util.HasPrefixIn(arg, "删除", "移除") {
		if !dao.IsSuperAdmin(c.GetSenderId()) {
			_, _ = c.Reply(e.PermissionDeniedNote)
			return
		}
		params := strings.Split(arg, "*")
		if len(params) < 2 {
			_, _ = c.Reply("参数格式错误! 正确格式: 删除*关键词")
			return
		}
		keyword := strings.TrimSpace(params[1])
		if err := dao.DeleteTopicByKeyword(keyword); err != nil {
			_, _ = c.Reply(err.Error())
		} else {
			_, _ = c.Reply(fmt.Sprintf("🗑 主题「%s」已删除！", keyword))
		}
		return
	}

	keywords, err := dao.LoadTopicKeywords()
	if err != nil {
		logger.Errorf("获取主题关键词列表失败: %v", err)
		_, _ = c.Reply(e.SystemErrorNote)
		return
	}

	var matchList []string
	searchKey := strings.TrimSpace(arg)
	for _, keyword := range keywords {
		if strings.Contains(keyword, searchKey) {
			matchList = append(matchList, keyword)
		}
		if keyword == searchKey {
			matchList = []string{keyword}
			break
		}
	}

	switch len(matchList) {
	case 0:
		_, _ = c.Reply("这个有点难，我还没学会呢")
	case 1:
		result, err := dao.GetTopicByKeyword(matchList[0])
		if err != nil {
			_, _ = c.Reply(e.SystemErrorNote)
			return
		}
		replyMsg := result.Value
		_, _ = c.Reply(replyMsg)
	default:
		msg := "这些主题你想看哪条呀?\n"
		msg += strings.Join(matchList, "\n")
		_, _ = c.Reply(msg)
	}
}

func processTopicImagesInContent(content, keyword string) (string, string, error) {
	re := regexp.MustCompile(`\[CQ:image,([^]]+)\]`)
	matches := re.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return content, "", nil
	}

	imagePaths := make([]string, 0)
	modifiedContent := content

	for i, match := range matches {
		params := match[1]
		urlRe := regexp.MustCompile(`url=([^,]+)`)
		urlMatch := urlRe.FindStringSubmatch(params)
		if len(urlMatch) < 2 {
			continue
		}
		imageURL := strings.ReplaceAll(urlMatch[1], "&amp;", "&")

		// 下载图片到本地
		timestamp := time.Now().UnixNano()
		fileName := fmt.Sprintf("%s_%d_%d.png", keyword, timestamp, i)
		savePath := filepath.Join(TopicImageSaveDir, fileName)
		if err := downloadImage(imageURL, savePath); err != nil {
			return "", "", fmt.Errorf("下载图片失败: %v", err)
		}
		imagePaths = append(imagePaths, savePath)

		// 生成 HTTP URL（替换本地路径为服务地址）
		httpURL := fmt.Sprintf("%s/topics/%s", TopicImageServerBaseURL, fileName)

		// 构造新参数：替换file字段为HTTP URL，删除url字段
		newParams := strings.Replace(params, urlMatch[0], "", 1)
		fileRe := regexp.MustCompile(`file=[^,]+`)
		if fileRe.MatchString(newParams) {
			newParams = fileRe.ReplaceAllString(newParams, "file="+httpURL)
		} else {
			newParams = "file=" + httpURL + "," + newParams
		}

		// 清理多余逗号
		newParams = regexp.MustCompile(`,{2,}`).ReplaceAllString(newParams, ",")
		newParams = strings.Trim(newParams, ",")

		// 替换整个图片块
		modifiedContent = strings.Replace(
			modifiedContent,
			match[0],
			fmt.Sprintf("[CQ:image,%s]", newParams),
			1,
		)
	}

	return modifiedContent, strings.Join(imagePaths, ";"), nil
}

func downloadImage(url, savePath string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 10; SM-G975F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回状态码: %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(savePath), os.ModePerm); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	file, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	return nil
}
