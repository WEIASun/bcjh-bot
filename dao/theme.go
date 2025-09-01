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

const CacheKeyThemeKeywords = "theme_keywords"
const CacheKeyThemeData = "theme_data_%s"

// LoadThemeKeywords 加载可用的主题数据关键词
func LoadThemeKeywords() ([]string, error) {
	var themies []string
	err := SimpleFindDataWithCache(CacheKeyThemeKeywords, &themies, func(dest interface{}) error {
		results := make([]database.Theme, 0)
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
	return themies, err
}

// GetThemeByKeyword 查询主题数据
func GetThemeByKeyword(keyword string) (database.Theme, error) {
	var theme database.Theme
	key := fmt.Sprintf(CacheKeyThemeData, keyword)
	err := SimpleFindDataWithCache(key, &theme, func(dest interface{}) error {
		_, err := DB.Where("keyword = ?", keyword).Get(dest)
		if err != nil {
			return err
		}
		return nil
	})
	return theme, err
}

// HasThemeKeyword 判断某个主题关键词是否存在
func HasThemeKeyword(keyword string) bool {
	keywords, err := LoadThemeKeywords()
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

// SearchStrategiesWithKeyword 根据关键词内容搜索
func SearchThemiesWithKeyword(keyword string) ([]database.Theme, error) {
	pattern := strings.ReplaceAll(keyword, "%", ".*")
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("描述格式有误 %v", err)
	}
	keywords, err := LoadThemeKeywords()
	if err != nil {
		return nil, fmt.Errorf("载入关键词数据失败 %v", err)
	}
	result := make([]database.Theme, 0)
	for i := range keywords {
		if re.MatchString(keywords[i]) {
			theme, err := GetThemeByKeyword(keywords[i])
			if err != nil {
				continue
			}
			result = append(result, theme)
		}
	}
	return result, nil
}

func CreateTheme(keyword string, value string) error {
	if keyword == "" || value == "" {
		return errors.New("未填写关键词或内容")
	}
	if HasThemeKeyword(keyword) {
		return errors.New("主题关键词已存在")
	}
	_, err := DB.Insert(&database.Theme{
		Keyword: keyword,
		Value:   value,
	})
	if err != nil {
		logger.Errorf("创建主题 %s 失败 %v", keyword, err)
		return errors.New(e.SystemErrorNote)
	}
	Cache.Delete(CacheKeyThemeKeywords)
	return nil
}

func UpdateTheme(keyword string, value string) error {
	if keyword == "" || value == "" {
		return errors.New("未填写关键词或内容")
	}
	if !HasThemeKeyword(keyword) {
		return errors.New("主题不存在，无法更新")
	}
	affected, err := DB.Where("keyword = ?", keyword).Update(&database.Theme{
		Keyword: keyword,
		Value:   value,
	})
	if err != nil {
		logger.Errorf("更新主题 %s 失败 %v", keyword, err)
		return errors.New(e.SystemErrorNote)
	}
	if affected == 0 {
		return errors.New("主题不存在")
	}
	Cache.Delete(CacheKeyThemeKeywords)
	Cache.Delete(fmt.Sprintf(CacheKeyThemeData, keyword))
	return nil
}

func DeleteThemeByKeyword(keyword string) error {
	if keyword == "" {
		return errors.New("未填写要移除的主题关键词")
	}
	if !HasThemeKeyword(keyword) {
		return errors.New("主题不存在，无法删除")
	}
	affected, err := DB.Where("keyword = ?", keyword).Delete(&database.Theme{})
	if err != nil {
		logger.Errorf("删除主题 %s 失败 %v", keyword, err)
		return errors.New(e.SystemErrorNote)
	}
	if affected == 0 {
		return errors.New("主题不存在")
	}
	Cache.Delete(CacheKeyThemeKeywords)
	Cache.Delete(fmt.Sprintf(CacheKeyThemeData, keyword))
	return nil
}
