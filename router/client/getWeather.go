package client

import (
	"AstraScheduleServerGo/model"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
)

// cache 缓存城市查询结果 (name, adm) -> LocationResp
var cache sync.Map

// cityLookup 查询城市位置信息
func cityLookup(name, adm, host, key string) (*model.LocationResp, error) {
	// 如果没有提供城市名，直接返回错误
	if name == "" {
		return nil, fmt.Errorf("城市名不能为空")
	}

	// 检查缓存
	cacheKey := fmt.Sprintf("%s_%s", name, adm)
	if cachedValue, ok := cache.Load(cacheKey); ok {
		cachedLoc := cachedValue.(*model.LocationResp)
		logrus.Infof(
			"Cache hit: name = %s, adm = %s -> id: %s | lat: %f, lon: %f",
			name, adm, cachedLoc.ID, cachedLoc.Lat, cachedLoc.Lon,
		)
		return cachedLoc, nil
	}

	// 构建请求 URL
	var url string
	if adm != "" {
		// 如果提供了省份信息
		url = fmt.Sprintf("https://%s/geo/v2/city/lookup?location=%s&adm=%s", host, name, adm)
	} else {
		// 如果没有提供省份信息
		url = fmt.Sprintf("https://%s/geo/v2/city/lookup?location=%s", host, name)
	}

	// 使用 resty 发起请求
	client := resty.New()
	resp, err := client.R().
		SetHeader("X-QW-Api-Key", key).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("请求API失败: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("API (%s) 请求失败，状态码: %d", url, resp.StatusCode())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("解析JSON响应失败: %w", err)
	}

	if code, ok := result["code"].(string); !ok || code != "200" {
		return nil, fmt.Errorf("API返回错误码: %v", result["code"])
	}

	// 获取第一个城市结果
	if results, ok := result["location"].([]interface{}); ok && len(results) > 0 {
		if location, ok := results[0].(map[string]interface{}); ok {
			id, idOk := location["id"].(string)
			latStr, latOk := location["lat"].(string)
			lonStr, lonOk := location["lon"].(string)
			name, nameOk := location["name"].(string)

			if idOk && latOk && lonOk && nameOk {
				// 将字符串格式的经纬度转换为 float64
				lat, err1 := strconv.ParseFloat(latStr, 64)
				lon, err2 := strconv.ParseFloat(lonStr, 64)

				if err1 == nil && err2 == nil {
					result := &model.LocationResp{
						ID:   id,
						Lat:  lat,
						Lon:  lon,
						Name: name,
					}

					// 存入缓存
					cache.Store(cacheKey, result)

					return result, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("未找到城市信息 (%s)", url)
}

// weatherLookup 查询指定位置的天气信息
func weatherLookup(location, host, key string) (*model.WeatherResp, error) {
	url := fmt.Sprintf("https://%s/v7/weather/now?location=%s", host, location)

	client := resty.New()
	resp, err := client.R().
		SetHeader("X-QW-Api-Key", key).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("请求天气API失败: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("天气API (%s) 请求失败，状态码: %d", url, resp.StatusCode())
	}

	var result model.WeatherResp
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("解析天气JSON响应失败: %w", err)
	}

	if result.Now.Temp == "" {
		return nil, fmt.Errorf("未获取到天气信息")
	}

	return &result, nil
}

// weatherLookupByName 查询天气信息
func weatherLookupByName(name, adm, host, key string) (*model.WeatherResp, error) {
	// 首先通过城市名称获取城市ID
	locationInfo, err := cityLookup(name, adm, host, key)
	if err != nil {
		return nil, fmt.Errorf("获取城市信息失败: %w", err)
	}

	// 使用城市ID查询天气
	return weatherLookup(locationInfo.ID, host, key)
}

// weatherWarningLookup 查询指定位置的天气预警信息
func weatherWarningLookup(lat, lon, host, key string) (*model.WarningResp, error) {
	url := fmt.Sprintf("https://%s/weatheralert/v1/current/%s/%s", host, lat, lon)

	client := resty.New()
	resp, err := client.R().
		SetHeader("X-QW-Api-Key", key).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("请求天气预警API失败: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("天气预警API (%s) 请求失败，状态码: %d", url, resp.StatusCode())
	}

	var result model.WarningResp
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("解析预警JSON响应失败: %w", err)
	}

	return &result, nil
}

// weatherWarningLookupByName 查询天气预警信息
func weatherWarningLookupByName(name, adm, host, key string) (*model.WarningResp, error) {
	// 首先通过城市名称获取城市信息
	locationInfo, err := cityLookup(name, adm, host, key)
	if err != nil {
		return nil, fmt.Errorf("获取城市信息失败: %w", err)
	}

	// 使用城市经纬度查询天气预警
	lat := fmt.Sprintf("%.5f", locationInfo.Lat) // 保留5位小数
	lon := fmt.Sprintf("%.5f", locationInfo.Lon) // 保留5位小数
	return weatherWarningLookup(lat, lon, host, key)
}

// getWeather 获取指定城市的天气信息
func getWeather(c *gin.Context, name, province string) {
	logrus.Infof("将查询 %s 省 %s 市的天气信息", province, name)
	for i := 0; i < 5; i++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("获取天气信息时发生 panic: %v", r)
				}
			}()

			// 从配置中获取 API 信息
			apiKey := model.Configs.APIKey
			host := apiKey.APIHost
			key := apiKey.Weather

			// 查找城市位置
			locationResp, err := cityLookup(name, province, host, key)
			if err != nil {
				logrus.Errorf("获取城市位置失败: %v", err)
				c.JSON(http.StatusNotFound, gin.H{
					"temp":       "404",
					"weat":       "不存在",
					"warning":    "",
					"brief_warn": "",
				})
				return
			}

			// 获取天气信息
			resp, err := weatherLookupByName(name, province, host, key)
			if err != nil {
				logrus.Errorf("获取天气信息失败: %v", err)
				return
			}

			// 获取天气预警信息
			warnResp, err := weatherWarningLookupByName(name, province, host, key)
			if err != nil {
				logrus.Errorf("获取天气预警信息失败: %v", err)
				warnResp = &model.WarningResp{Alerts: []model.Alert{}}
			}

			// 处理预警信息
			var warnParts []string
			var briefWarnParts []string
			for _, alert := range warnResp.Alerts {
				warnParts = append(warnParts, strings.ReplaceAll(alert.Description, "\n", ""))
				briefWarnParts = append(briefWarnParts, strings.ReplaceAll(alert.Headline, "\n", ""))
			}

			warn := strings.Join(warnParts, "；")
			briefWarn := strings.Join(briefWarnParts, "；")

			location := locationResp.Name

			// 记录日志
			logrus.Infof(
				"获取 %s/%s（%s） 的天气信息，T: %s, W: %s, Warning: %s, Brief: %s, Wind: %s (%s级)",
				province, name, location, resp.Now.Temp, resp.Now.Text, warn, briefWarn,
				resp.Now.WindDir, resp.Now.WindScale,
			)

			// 返回响应
			c.JSON(http.StatusOK, model.WeatherResponse{
				Where:     location,
				Temp:      resp.Now.Temp,
				Weat:      resp.Now.Text,
				Wind:      resp.Now.WindDir,
				WindPower: resp.Now.WindScale,
				Warn:      warn,
				BriefWarn: briefWarn,
			})
			return
		}()

		// 如果响应已经写入，说明请求已处理（成功或失败），直接返回
		if c.Writer.Written() {
			return
		}
	}

	// 如果所有重试都失败，返回错误响应

	logrus.Errorf("获取 %s/%s 的天气信息失败，超过最大重试次数", province, name)

	c.JSON(http.StatusBadGateway, gin.H{ // 502
		"error": "获取天气信息失败，超过最大重试次数，可能是上游服务器异常，或是本服务器存在网络波动",
	})
}

// GetWeatherWithProvince 获取指定省份和城市的天气信息
func GetWeatherWithProvince(c *gin.Context) {
	name1 := c.Param("name1") // 省
	name2 := c.Param("name2") // 市
	if name2 == "" {
		getWeather(c, name1, "")
	} else {
		getWeather(c, name2, name1)
	}
}

// GetWeatherWithCity 获取指定省份和城市的天气信息
func GetWeatherWithCity(c *gin.Context) {
	name1 := c.Param("name1") // 市
	getWeather(c, name1, "")
}

// GetWeatherWithCFHeader 通过 Cloudflare 请求头获取天气信息
func GetWeatherWithCFHeader(c *gin.Context) {
	cfCity := c.GetHeader("CF-IPCity")
	cfRegion := c.GetHeader("CF-Region")

	if cfCity == "" {
		logrus.Errorf("无法通过请求头获取城市信息")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无法通过请求头获取城市信息，请确保请求经过 Cloudflare",
		})
		return
	}

	province := cfRegion
	if province == "" {
		province = ""
	}

	getWeather(c, cfCity, province)
}
