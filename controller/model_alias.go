package controller

import (
	"encoding/json"
	"net/http"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

func GetModelAliases(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    setting.GetModelAliases(),
	})
}

func UpdateModelAliases(c *gin.Context) {
	var aliases []setting.ModelAlias
	if err := c.ShouldBindJSON(&aliases); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	jsonStr, err := marshalModelAliases(aliases)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.UpdateOption("ModelAliases", jsonStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func AddModelAlias(c *gin.Context) {
	var alias setting.ModelAlias
	if err := c.ShouldBindJSON(&alias); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	aliases := setting.GetModelAliases()
	for _, a := range aliases {
		if a.Alias == alias.Alias {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": "alias already exists, use PUT to update"})
			return
		}
	}
	aliases = append(aliases, alias)
	jsonStr, err := marshalModelAliases(aliases)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.UpdateOption("ModelAliases", jsonStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func DeleteModelAlias(c *gin.Context) {
	aliasName := c.Param("alias")
	aliases := setting.GetModelAliases()
	var updated []setting.ModelAlias
	found := false
	for _, a := range aliases {
		if a.Alias == aliasName {
			found = true
			continue
		}
		updated = append(updated, a)
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "alias not found"})
		return
	}
	jsonStr, err := marshalModelAliases(updated)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.UpdateOption("ModelAliases", jsonStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func marshalModelAliases(aliases []setting.ModelAlias) (string, error) {
	if aliases == nil {
		aliases = []setting.ModelAlias{}
	}
	bytes, err := json.Marshal(aliases)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
