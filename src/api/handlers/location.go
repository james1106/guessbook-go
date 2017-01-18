package handlers

import (
	"api/models"
	"github.com/gin-gonic/gin"
	"net/http"
	"api/services"
)

func Location(c *gin.Context) {
	db := services.InitDB()
	defer db.Close()
	location := models.GeoPoint{Lng: 43.76857094631136, Lat: 11.292383687705296}
	p := models.Location{Ponit: location}

	db.Create(&p)

	var res models.Location
	db.First(&res)

	c.JSON(http.StatusOK, &res)

}
