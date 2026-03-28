package web

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
)

func GetStatistic(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"weather_error":              0,
		"websocket_disconnect":       gin.H{},
		"websocket_disconnect_count": 0,
		"clients":                    []string{},
		"clients_count":              0,
	})
}

func listSchools() ([]string, error) {
	type row struct{ School string }
	rows := make([]row, 0)
	err := db.GetDB().Model(&dbTable.Schedule{}).Distinct("school").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.School)
	}
	sort.Strings(out)
	return out, nil
}

func listGrades(school string) ([]string, error) {
	type row struct{ Grade string }
	rows := make([]row, 0)
	err := db.GetDB().Model(&dbTable.Schedule{}).Where("school = ?", school).Distinct("grade").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Grade)
	}
	sort.Strings(out)
	return out, nil
}

func listClasses(school, grade string) ([]string, error) {
	type row struct{ Class string }
	rows := make([]row, 0)
	err := db.GetDB().Model(&dbTable.Schedule{}).Where("school = ? AND grade = ?", school, grade).Distinct("class").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Class)
	}
	sort.Strings(out)
	return out, nil
}

func GetMenu(c *gin.Context) {
	menu := gin.H{"data": []gin.H{{"to": "/", "text": "总览", "key": "go-back-home", "children": nil}, {"to": "/autorun", "text": "自动任务", "key": "autorun", "children": nil}, {"to": "/countdown", "text": "倒数日", "key": "countdown", "children": nil}}}
	data := menu["data"].([]gin.H)
	schools, err := listSchools()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, school := range schools {
		grades, _ := listGrades(school)
		gradeChildren := make([]gin.H, 0)
		for _, grade := range grades {
			classes, _ := listClasses(school, grade)
			children := []gin.H{
				{"to": "/config/" + school + "/" + grade + "/subjects", "text": "课程设置", "key": "school-" + school + "-grade-" + grade + "-subjects", "children": nil},
				{"to": "/config/" + school + "/" + grade + "/timetable", "text": "作息设置", "key": "school-" + school + "-grade-" + grade + "-timetable", "children": nil},
			}
			for _, classNumber := range classes {
				children = append(children, gin.H{
					"text": classNumber + " 班",
					"key":  "school-" + school + "-grade-" + grade + "-class-" + classNumber,
					"raw":  classNumber,
					"children": []gin.H{
						{"to": "/config/" + school + "/" + grade + "/" + classNumber + "/schedule", "text": "课表设置", "key": "school-" + school + "-grade-" + grade + "-class-" + classNumber + "-schedule", "children": nil},
						{"to": "/config/" + school + "/" + grade + "/" + classNumber + "/settings", "text": "通用设置", "key": "school-" + school + "-grade-" + grade + "-class-" + classNumber + "-settings", "children": nil},
						{"to": "/countdown", "text": "倒数日设置", "key": "school-" + school + "-grade-" + grade + "-class-" + classNumber + "-countdown", "children": nil},
					},
				})
			}
			gradeChildren = append(gradeChildren, gin.H{"text": grade + " 级", "key": "school-" + school + "-grade-" + grade, "raw": grade, "children": children})
		}
		data = append(data, gin.H{"text": school + " 学校", "key": "school-" + school, "raw": school, "children": gradeChildren})
	}
	menu["data"] = data
	c.JSON(http.StatusOK, menu)
}

func GetStructure(c *gin.Context) {
	schools, err := listSchools()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0)
	for _, school := range schools {
		grades, _ := listGrades(school)
		gradeNodes := make([]gin.H, 0)
		for _, grade := range grades {
			classes, _ := listClasses(school, grade)
			classNodes := make([]gin.H, 0)
			for _, classNumber := range classes {
				classNodes = append(classNodes, gin.H{"text": classNumber, "children": nil})
			}
			gradeNodes = append(gradeNodes, gin.H{"text": grade, "children": classNodes})
		}
		out = append(out, gin.H{"text": school, "children": gradeNodes})
	}
	c.JSON(http.StatusOK, out)
}
