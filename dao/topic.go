package dao

import (
	"bcjh-bot/model/database"
	"bcjh-bot/util/e"
	"bcjh-bot/util/logger"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const CacheKeyTopicKeywords = "topic_keywords"
const CacheKeyTopicData = "topic_data_%s"

// LoadTopicKeywords 加载可用的主题数据关键词
func LoadTopicKeywords() ([]string, error) {
	var topics []string
	err := SimpleFindDataWithCache(CacheKeyTopicKeywords, &topics, func(dest interface{}) error {
		results := make([]database.Topic, 0)
		err := DB.Cols("keyword").Find(&results)
		if err != nil {
			return err
		}
		keywords := make([]string, 0, len(results))
		for i := range results {
			keywords = append(keywords, results[i].Keyword)
		}
		*dest.(*[]string) = keywords
		return nil
	})
	return topics, err
}

// GetTopicByKeyword 查询主题数据
func GetTopicByKeyword(keyword string) (database.Topic, error) {
	var topic database.Topic
	key := fmt.Sprintf(CacheKeyTopicData, keyword)
	err := SimpleFindDataWithCache(key, &topic, func(dest interface{}) error {
		_, err := DB.Where("keyword = ?", keyword).Get(dest)
		if err != nil {
			return err
		}
		return nil
	})
	return topic, err
}

// HasTopicKeyword 判断某个主题关键词是否存在
func HasTopicKeyword(keyword string) bool {
	keywords, err := LoadTopicKeywords()
	if err != nil {
		logger.Errorf("载入关键词数据列表出错 %v", err)
		return false
	}
	for i := range keywords {
		if keywords[i] == keyword {
			return true
		}
	}
	return false
}

// SearchTopicsWithKeyword 根据关键词内容搜索
func SearchTopicsWithKeyword(keyword string) ([]database.Topic, error) {
	pattern := strings.ReplaceAll(keyword, "%", ".*")
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("描述格式有误 %v", err)
	}
	keywords, err := LoadTopicKeywords()
	if err != nil {
		return nil, fmt.Errorf("载入关键词数据失败 %v", err)
	}
	result := make([]database.Topic, 0)
	for i := range keywords {
		if re.MatchString(keywords[i]) {
			topic, err := GetTopicByKeyword(keywords[i])
			if err != nil {
				continue
			}
			result = append(result, topic)
		}
	}
	return result, nil
}

// CreateTopic 创建新的主题
func CreateTopic(keyword, value, imagePaths string) error {
	if keyword == "" || value == "" {
		return errors.New("未填写关键词或内容")
	}
	if HasTopicKeyword(keyword) {
		return errors.New("主题关键词已存在")
	}
	_, err := DB.Insert(&database.Topic{
		Keyword: keyword,
		Value:   value,
		Image:   imagePaths, // 保存分号分隔的本地路径
	})
	if err != nil {
		logger.Errorf("创建主题 %s 失败 %v", keyword, err)
		return errors.New(e.SystemErrorNote)
	}
	Cache.Delete(CacheKeyTopicKeywords)
	return nil
}

// UpdateTopic 更新主题（修改参数类型）
func UpdateTopic(keyword, value, imagePaths string) error {
	if keyword == "" || value == "" {
		return errors.New("未填写关键词或内容")
	}
	if !HasTopicKeyword(keyword) {
		return errors.New("主题不存在，无法更新")
	}
	affected, err := DB.Where("keyword = ?", keyword).Update(&database.Topic{
		Keyword: keyword,
		Value:   value,
		Image:   imagePaths, // 保存分号分隔的本地路径
	})
	if err != nil {
		logger.Errorf("更新主题 %s 失败 %v", keyword, err)
		return errors.New(e.SystemErrorNote)
	}
	if affected == 0 {
		return errors.New("主题不存在")
	}
	Cache.Delete(CacheKeyTopicKeywords)
	Cache.Delete(fmt.Sprintf(CacheKeyTopicData, keyword))
	return nil
}

// DeleteTopicByKeyword 根据关键词删除主题
func DeleteTopicByKeyword(keyword string) error {
	if keyword == "" {
		return errors.New("未填写要移除的主题关键词")
	}
	if !HasTopicKeyword(keyword) {
		return errors.New("主题不存在，无法删除")
	}
	affected, err := DB.Where("keyword = ?", keyword).Delete(&database.Topic{})
	if err != nil {
		logger.Errorf("删除主题 %s 失败 %v", keyword, err)
		return errors.New(e.SystemErrorNote)
	}
	if affected == 0 {
		return errors.New("主题不存在")
	}
	Cache.Delete(CacheKeyTopicKeywords)
	Cache.Delete(fmt.Sprintf(CacheKeyTopicData, keyword))
	return nil
}
