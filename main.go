package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/360EntSecGroup-Skylar/excelize"
)

const roomInfoURL string = "https://api.live.bilibili.com/room/v1/Room/room_init?id=%v"
const gruadInfoURL string = "https://api.live.bilibili.com/xlive/app-room/v1/guardTab/topList?roomid=%v&ruid=%v&page_size=10&page=%v"

func main() {
	defer func() {
		var in string
		fmt.Println("")
		fmt.Println("按 回车 键退出...")
		fmt.Scanln(&in)
	}()

	fmt.Print("请输入BiliBili直播房间号: ")
	reader := bufio.NewScanner(os.Stdin)
	roomIDStr := ""
	if reader.Scan() {
		roomIDStr = reader.Text()
	}
	roomID, err := strconv.Atoi(roomIDStr)
	if err != nil {
		fmt.Println("请输入正确的房间号")
		return
	}
	if roomID <= 0 {
		fmt.Println("房间号错误!")
		return
	}
	roomInfo, err := getRoomInfo(roomID)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "用户昵称")
	f.SetCellValue("Sheet1", "B1", "用户ID")
	f.SetCellValue("Sheet1", "C1", "船票等级")

	var wg sync.WaitGroup
	writeQueue := make(chan *model, 100)

	wg.Add(1)
	go func(q chan<- *model) {
		defer wg.Done()
		defer close(q)
		page, now := 1, 1
		for now <= page {
			resBytes, err := httpSend(fmt.Sprintf(gruadInfoURL, roomInfo.Data.RoomID, roomInfo.Data.UserID, now))
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			guardInfo := &model{}
			json.Unmarshal(resBytes, guardInfo)
			if roomInfo.Code != 0 {
				fmt.Println("数据获取错误")
				return
			}
			q <- guardInfo
			page = guardInfo.Data.Info.Page
			now++
		}
	}(writeQueue)

	wg.Add(1)
	go func(q <-chan *model) {
		defer wg.Done()
		line := 2
		readTop := false
		for {
			guardInfo, ok := <-q
			if !ok {
				return
			}
			if !readTop {
				for _, info := range guardInfo.Data.Top3 {
					fmt.Println("读取...", info.UserName)
					f.SetCellValue("Sheet1", fmt.Sprintf("A%v", line), info.UserName)
					f.SetCellValue("Sheet1", fmt.Sprintf("B%v", line), strconv.Itoa(info.UserID))
					f.SetCellValue("Sheet1", fmt.Sprintf("C%v", line), info.GuardLevel)
					line++
				}
				readTop = true
			}
			for _, info := range guardInfo.Data.List {
				fmt.Println("读取...", info.UserName)
				f.SetCellValue("Sheet1", fmt.Sprintf("A%v", line), info.UserName)
				f.SetCellValue("Sheet1", fmt.Sprintf("B%v", line), strconv.Itoa(info.UserID))
				f.SetCellValue("Sheet1", fmt.Sprintf("C%v", line), info.GuardLevel)
				line++
			}
		}
	}(writeQueue)

	// 等待统计写入完成
	wg.Wait()

	// 保存文件
	fileName := fmt.Sprintf("船员-%v.xlsx", time.Now().Format("20060102150405"))
	fmt.Println("")
	if err = f.SaveAs(fileName); err != nil {
		fmt.Println("保存失败，", err.Error())
	} else {
		fmt.Println("读取完成，数据已保存到", fileName)
	}
}

func getRoomInfo(roomID int) (*roomInfoResult, error) {
	resRoom, err := httpSend(fmt.Sprintf(roomInfoURL, roomID))
	if err != nil {
		return nil, err
	}
	roomInfo := &roomInfoResult{}
	json.Unmarshal(resRoom, roomInfo)
	if roomInfo.Code != 0 {
		return nil, errors.New("房间不正确")
	}
	return roomInfo, nil
}

func httpSend(url string) ([]byte, error) {
	tr := &http.Transport{ //解决x509: certificate signed by unknown authority
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// 房间信息
type roomInfoResult struct {
	Code int           `json:"code"`
	Data *roomInfoData `json:"data"`
}

// 房间数据
type roomInfoData struct {
	RoomID int `json:"room_id"`
	UserID int `json:"uid"`
}

type model struct {
	Code int `json:"code"`
	Data struct {
		Info struct {
			Num  int `json:"num"`
			Page int `json:"page"`
			Now  int `json:"now"`
		} `json:"info"`
		List []struct {
			UserID     int    `json:"uid"`
			UserName   string `json:"username"`
			GuardLevel int    `json:"guard_level"`
		} `json:"list"`
		Top3 []struct {
			UserID     int    `json:"uid"`
			UserName   string `json:"username"`
			GuardLevel int    `json:"guard_level"`
		} `json:"top3"`
	} `json:"data"`
}
