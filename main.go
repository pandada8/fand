package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
)

type Endpoint struct {
	DHCPExpiry time.Time
	IP         string
	MAC        string
	Hostname   string
	IsFan      bool
	IsMe       bool
}

func getFanMembers() (ret map[string]struct{}, err error) {
	cmd := exec.Command("nft", "-j", "list", "set", "ip", "clash", "fan")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	nodes, err := sonic.Get(output, "nftables", 1, "set", "elem")
	if err != nil {
		return nil, err
	}
	arr, err := nodes.Array()
	if err != nil {
		return nil, err
	}
	ret = make(map[string]struct{})
	for _, node := range arr {
		ret[node.(string)] = struct{}{}
	}
	return
}

// parseEndpoints parses /var/lib/misc/dnsmasq.leases and returns a list of endpoints
func parseEndpoints(clientIP string) (ret []Endpoint) {
	file, err := os.Open("/var/lib/misc/dnsmasq.leases")
	if err != nil {
		// Handle error appropriately
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 5 {
			dhcpTime, _ := strconv.ParseInt(parts[0], 10, 64)
			ret = append(ret, Endpoint{
				DHCPExpiry: time.Unix(dhcpTime, 0).UTC(),
				MAC:        parts[1],
				IP:         parts[2],
				Hostname:   parts[3],
			})
		}
	}

	fan, _ := getFanMembers()
	for i := range ret {
		if _, ok := fan[ret[i].IP]; ok {
			ret[i].IsFan = true
		}
		if ret[i].IP == clientIP {
			ret[i].IsMe = true
		}
	}
	return
}

func renderIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "", index(parseEndpoints(c.ClientIP())))
}

func main() {
	var listen = flag.String("listen", ":8080", "listen address")
	flag.Parse()
	router := gin.Default()
	router.HTMLRender = &TemplRender{}
	router.GET("/", renderIndex)
	router.POST("/fan/:ip", func(c *gin.Context) {
		ip := c.Param("ip")
		cmd := exec.Command("nft", "add", "element", "ip", "clash", "fan", "{", ip, "}")
		cmd.Run()
		renderIndex(c)
	})
	router.POST("/unfan/:ip", func(c *gin.Context) {
		ip := c.Param("ip")
		cmd := exec.Command("nft", "delete", "element", "ip", "clash", "fan", "{", ip, "}")
		cmd.Run()
		renderIndex(c)
	})
	fmt.Println("listening on", *listen)
	router.Run(*listen)
}
