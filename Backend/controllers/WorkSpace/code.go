package workSpace

import (
	"TalkHive/global"
	"TalkHive/models"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 我的代码！！！

// SearchCode - 获取用户的代码列表√
func SearchCode(c *gin.Context) {
	// 1. 获取请求参数中的 id
	userID := c.GetHeader("User-Id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	// 2. 定义结果集，用于存储筛选后的代码数据
	var codes []models.Codes

	// 3. 查询数据库，筛选出符合条件的数据：account_id = id 且 is_show = true，按时间降序排序
	if err := global.Db.Model(&models.Codes{}).
		Where("account_id = ? AND is_show = ?", global.ParseUint(userID), true).
		Order("save_time DESC"). // 按时间降序排序
		Find(&codes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch code list"})
		return
	}

	// 4. 返回筛选后的结果集
	var result []map[string]interface{}
	for _, code := range codes {
		result = append(result, map[string]interface{}{
			"code_id":            code.CodeID,
			"code_name":          code.Name,
			"last_modified_time": code.SaveTime.Format("2006-01-02 15:04"), // 使用正确的时间格式
			"Suffix":             code.Suffix,
		})
	}

	// 5. 返回 JSON 格式的结果
	c.JSON(http.StatusOK, result)
}

// CreateCode - 新建并保存代码文件√
func CreateCode(c *gin.Context) {
	// 1. 接收上传的文件
	//file, err := c.FormFile("file")
	//if err != nil {
	//	c.JSON(http.StatusBadRequest, gin.H{"message": "No file uploaded", "error": err.Error()})
	//	return
	//}

	// 2. 接收表单其他参数
	var req struct {
		Name   string `json:"Name" binding:"required"`
		Suffix string `json:"Suffix" bingding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid form data", "error": err.Error()})
		return
	}

	// 获取用户ID
	userID := c.GetHeader("User-Id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	// 4.确保文件路径安全并添加后缀
	sanitizedNoteName := strings.ReplaceAll(req.Name, "/", "_")
	sanitizedNoteName = strings.ReplaceAll(sanitizedNoteName, "\\", "_")
	rootDir := "D:/TalkHive/Codes/"
	filePath := filepath.Join(rootDir, sanitizedNoteName+req.Suffix)

	// 确保目录存在
	if err := os.MkdirAll(rootDir, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create directory", "error": err.Error()})
		return
	}

	// 5.创建空白文件
	file, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create blank note file", "error": err.Error()})
		return
	}
	defer file.Close()

	// 6. 将文件信息存入数据库
	code := models.Codes{
		Name:      req.Name,
		SaveTime:  time.Now(),
		CachePath: sanitizedNoteName + req.Suffix, // 保存相对路径
		Suffix:    req.Suffix,
		AccountID: global.ParseUint(userID), // 将 user_id 转为 uint,
		IsShow:    true,
	}

	if err := global.Db.Create(&code).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save code metadata", "error": err.Error()})
		return
	}

	// 7. 返回文件元信息
	c.JSON(http.StatusOK, gin.H{
		"code_id":            code.CodeID,
		"code_name":          code.Name,
		"last_modified_time": code.SaveTime.Format("2006-01-02 15:04"),
		"Suffix":             code.Suffix,
	})
}

// GetCode - 获取代码文件
func GetCode(c *gin.Context) {
	// 1. 从请求中获取 code_id 和 is_preview 参数
	var req struct {
		CodeID uint `json:"code_id" binding:"required"`
	}

	// 绑定请求数据
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request data", "error": err.Error()})
		return
	}

	//userID := c.Param("id")
	userID := c.GetHeader("User-Id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	// 2. 数据库查询：查找指定 CodeID 且 IsShow = true 的记录
	var code models.Codes
	if err := global.Db.Model(&models.Codes{}).Where("code_id = ? AND is_show = ?",
		req.CodeID, true).First(&code).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Code not found or not visible"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database error", "error": err.Error()})
		return
	}

	// 3. 拼接文件的完整路径
	rootDir := "D:/TalkHive/Codes/" // 默认根目录
	filePath := filepath.Join(rootDir, code.CachePath)

	// 4. 验证文件是否存在
	file, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to open file", "error": err.Error()})
		return
	}
	defer file.Close()

	// 5. 设置正确的 HTTP 响应头（根据是否预览）
	suffix := code.Suffix
	contentType := getContentTypeBySuffix(suffix) // 根据后缀获取 Content-Type
	c.Header("Content-Type", contentType)

	// 6. 返回文件流
	if _, err := io.Copy(c.Writer, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to send file", "error": err.Error()})
		return
	}
}

// getContentTypeBySuffix - 根据文件后缀返回对应的 Content-Type
func getContentTypeBySuffix(suffix string) string {
	switch suffix {
	case ".js":
		return "application/javascript"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".json":
		return "application/json"
	case ".txt":
		return "text/plain"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream" // 默认二进制流
	}
}

// EditCode - 编辑代码文件（包括修改名称）
func EditCode(c *gin.Context) {
	// 接收 JSON 数据
	var requestData struct {
		CodeID   uint   `json:"code_id" binding:"required"`   // 代码文件ID
		CodeName string `json:"code_name" binding:"required"` // 代码文件名称
		Suffix   string `json:"suffix" binding:"required"`    // 文件后缀
		Content  string `json:"content" binding:"required"`   // 代码内容
	}

	// 绑定 JSON 数据
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// 获取用户ID
	userID := c.GetHeader("User-Id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	// 1. 检查数据库中是否有匹配的记录
	var code models.Codes
	if err := global.Db.Model(&models.Codes{}).Where("code_id = ? AND account_id = ? AND is_show = ?",
		requestData.CodeID, global.ParseUint(userID), true).First(&code).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Code file not found"})
		return
	}

	// 2. 更新数据库中的名称和后缀
	//if err := global.Db.Model(&models.Codes{}).Where("code_id = ?", requestData.CodeID).
	//	Updates(map[string]interface{}{
	//		"name":   requestData.CodeName,
	//		"suffix": requestData.Suffix,
	//	}).Error; err != nil {
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update code metadata"})
	//	return
	//}

	// 更新笔记名称为 note_name
	if err := global.Db.Model(&code).Update("name", requestData.CodeName).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update note name"})
		return
	}

	// 2. 更新 Notes 表，将 Type 修改为 Type
	if err := global.Db.Model(&code).Update("suffix", requestData.Suffix).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update note type"})
		return
	}

	// 3. 定义文件存储路径
	savePath := fmt.Sprintf("D:/TalkHive/Codes/%s", code.CachePath)

	// 4. 将文本内容保存到指定路径
	if err := os.WriteFile(savePath, []byte(requestData.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file content"})
		return
	}

	// 5. 更新数据库中的保存时间
	code.SaveTime = time.Now()
	if err := global.Db.Save(&code).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update code metadata"})
		return
	}

	// 返回元信息给前端
	c.JSON(http.StatusOK, gin.H{
		"code_id":    code.CodeID,
		"name":       code.Name,
		"save_time":  code.SaveTime.Format("2006-01-02 15:04"),
		"suffix":     code.Suffix,
		"cache_path": code.CachePath,
		"user_id":    code.AccountID,
		"is_show":    code.IsShow,
	})
}

// ShareCode - 分享代码文件√
func ShareCode(c *gin.Context) {
	// 获取请求参数
	var requestData struct {
		CodeID uint `json:"code_id" binding:"required"`
		FdID   uint `json:"fd_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// 获取用户ID
	//userID := c.Param("id")
	userID := c.GetHeader("User-Id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	// 第一步：验证用户和好友关系
	var contact models.Contacts
	if err := global.Db.Model(&models.Contacts{}).Where("owner_id = ? AND contact_id = ?",
		userID, requestData.FdID).First(&contact).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"message": "You cannot share files with this user"})
		return
	}

	// 第二步：检查代码文件是否存在并且可分享
	var code models.Codes
	if err := global.Db.Model(&models.Codes{}).Where("code_id = ? AND is_show = ?",
		requestData.CodeID, true).First(&code).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Code file not found or not shareable"})
		return
	}

	// 第三步：新建分享记录，将文件分享给好友
	newCode := models.Codes{
		Name:      code.Name,
		SaveTime:  time.Now(),
		CachePath: code.CachePath,
		Suffix:    code.Suffix,
		AccountID: requestData.FdID, // 分享给的好友ID
		IsShow:    true,
	}
	if err := global.Db.Create(&newCode).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to share code file"})
		return
	}

	// 返回成功信息
	c.JSON(http.StatusOK, gin.H{
		"message": "Code shared successfully",
	})
}

// ChangeName - 修改代码名称√
func ChangeName(c *gin.Context) {
	// 获取请求参数
	var requestData struct {
		CodeID  uint   `json:"code_id" binding:"required"`
		OldName string `json:"old_code_name" binding:"required"`
		NewName string `json:"new_code_name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// 获取用户ID
	//userID := c.Param("id")
	userID := c.GetHeader("User-Id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	// 更新数据库
	if err := global.Db.Model(&models.Codes{}).Where("code_id = ?", requestData.CodeID).
		Update("name", requestData.NewName).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update code name"})
		return
	}

	// 返回成功信息
	c.JSON(http.StatusOK, gin.H{
		"message": "Code name updated successfully",
	})
}

// DeleteCode - 删除代码文件
func DeleteCode(c *gin.Context) {
	var code struct {
		CodeID uint `json:"code_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// 获取用户ID
	//userID := c.Param("id")
	userID := c.GetHeader("User-Id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	// 1.检查Codes表中是否存在指定代码文件
	var codes models.Codes
	if err := global.Db.Model(&models.Codes{}).Where("account_id = ? AND code_id = ?",
		userID, code.CodeID).First(&codes).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Note not found"})
		return
	}

	// 2. 更新 is_show 字段为 false（表示移动到回收站）
	codes.IsShow = false
	if err := global.Db.Save(&codes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update note status"})
		return
	}

	// 3. 向 Recycle 表插入回收站记录
	recycle := models.Recycle{
		RecycleID:   codes.CodeID, // 使用笔记ID作为回收ID（根据需求可以修改为自增）
		RecycleType: "code",
		AccountID:   codes.AccountID,
		RecycleTime: time.Now(), // 当前时间
	}
	if err := global.Db.Model(&models.Recycle{}).Create(&recycle).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add note to recycle bin"})
		return
	}

	// 4. 返回操作成功信息
	c.JSON(http.StatusOK, gin.H{"message": "Note deleted and moved to recycle bin successfully"})

}
