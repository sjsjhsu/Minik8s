package handler

import (
	"encoding/json"
	"fmt"
	"minik8s/apiobjects"
	"minik8s/apiserver/src/etcd"
	"minik8s/apiserver/src/route"
	"net/http"

	"github.com/gin-gonic/gin"
)

func NodeGetHandler(c *gin.Context) {
	var nodes []*apiobjects.Node
	values, err := etcd.Get_prefix(route.NodePath)
	if err != nil {
		fmt.Println(err)
	}
	for _, value := range values {
		var node apiobjects.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			fmt.Println(err)
		}
		nodes = append(nodes, &node)
	}
	c.JSON(http.StatusOK, nodes)
}
func PodGetWithNamespaceHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	var pods []*apiobjects.Pod
	values, err := etcd.Get_prefix(route.PodPath + "/" + namespace)
	if err != nil {
		fmt.Println(err)
	}
	for _, value := range values {
		var pod apiobjects.Pod
		err := json.Unmarshal([]byte(value), &pod)
		if err != nil {
			fmt.Println(err)
		}
		pods = append(pods, &pod)
	}
	c.JSON(http.StatusOK, pods)
}
func PodGetDetailHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("name")
	url := "/api/binding" + "/" + namespace + "/" + podName
	val, _ := etcd.Get(url)
	var binding apiobjects.NodePodBinding
	if val == "" {
		c.JSON(http.StatusOK, binding)
	}
	err := json.Unmarshal([]byte(val), &binding)
	if err != nil {
		fmt.Println(err)
	}
	c.JSON(http.StatusOK, binding)
}