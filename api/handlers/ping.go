
package handlers

import "github.com/gin-gonic/gin"

import "fmt"

func HandlePing(c *gin.Context){
	result := getPongMessage()

	// silly print test
	nums := []int{
		6,
		8,
		9,
	}
	for _, num := range nums {
		fmt.Printf("%d \n", num)
	}

	c.JSON(200, gin.H{
		"message": result,
	})
}

func getPongMessage() string {
	return "pong"
}