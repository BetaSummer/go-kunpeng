package service

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang/protobuf/proto"

	"github.com/withlin/canal-go/client"
	protocol "github.com/withlin/canal-go/protocol"

	cc "go-kunpeng/config/canal"
)

func StartCanalClient() {
	connector := client.NewSimpleCanalConnector(cc.Address, cc.Port, cc.Username, cc.Password, cc.Destination, cc.SoTimeOut, cc.IdleTimeOut)
	err := connector.Connect()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	rc, err := CreateRedisClient()
	if err != nil {
		log.Println(err)
		return
	}
	defer rc.Close()

	// https://github.com/alibaba/canal/wiki/AdminGuide
	//mysql 数据解析关注的表，Perl正则表达式.
	//
	//多个正则之间以逗号(,)分隔，转义符需要双斜杠(\\)
	//
	//常见例子：
	//
	//  1.  所有表：.*   or  .*\\..*
	//	2.  canal schema下所有表： canal\\..*
	//	3.  canal下的以canal打头的表：canal\\.canal.*
	//	4.  canal schema下的一张表：canal\\.test1
	//  5.  多个规则组合使用：canal\\..*,mysql.test1,mysql.test2 (逗号分隔)

	err = connector.Subscribe(".*\\..*")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	for {
		message, err := connector.Get(100, nil, nil)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		batchId := message.Id
		if batchId == -1 || len(message.Entries) <= 0 {
			time.Sleep(cc.PollingInterval * time.Millisecond)
			// fmt.Println("===暂时没有数据更新===")
			continue
		}

		// 打数据变更log
		go printEntry(message.Entries)
		// 数据处理协程
		go handleData(message.Entries, rc)
	}
}

func handleData(es []protocol.Entry, c *redis.Client) {

	for _, entry := range es {
		if entry.GetEntryType() == protocol.EntryType_TRANSACTIONBEGIN || entry.GetEntryType() == protocol.EntryType_TRANSACTIONEND {
			continue
		}

		rowChange := new(protocol.RowChange)
		err := proto.Unmarshal(entry.GetStoreValue(), rowChange)
		if err != nil {
			log.Println(err.Error())
			continue
		}

		header := entry.GetHeader()

		switch header.GetTableName() {
		case "common_user_info":
			break
		case "common_user_role_relation":
			break
		case "common_role":
			break
		case "organization_member":
			break
		case "common_user":
			break
		case "activity_record":
			break
		case "activity":
			break
		default:
			break
		}
	}
}

func printEntry(entrys []protocol.Entry) {

	for _, entry := range entrys {
		if entry.GetEntryType() == protocol.EntryType_TRANSACTIONBEGIN || entry.GetEntryType() == protocol.EntryType_TRANSACTIONEND {
			continue
		}
		rowChange := new(protocol.RowChange)

		err := proto.Unmarshal(entry.GetStoreValue(), rowChange)
		checkError(err)
		eventType := rowChange.GetEventType()
		header := entry.GetHeader()
		fmt.Println(fmt.Sprintf("================> binlog[%s : %d],name[%s,%s], eventType: %s", header.GetLogfileName(), header.GetLogfileOffset(), header.GetSchemaName(), header.GetTableName(), header.GetEventType()))

		for _, rowData := range rowChange.GetRowDatas() {
			if eventType == protocol.EventType_DELETE {
				printColumn(rowData.GetBeforeColumns())
			} else if eventType == protocol.EventType_INSERT {
				printColumn(rowData.GetAfterColumns())
			} else {
				fmt.Println("-------> before")
				printColumn(rowData.GetBeforeColumns())
				fmt.Println("-------> after")
				printColumn(rowData.GetAfterColumns())
			}
		}
	}
}

func printColumn(columns []*protocol.Column) {
	for _, col := range columns {
		fmt.Println(fmt.Sprintf("%s : %s  update= %t", col.GetName(), col.GetValue(), col.GetUpdated()))
	}
}

func checkError(err error) {
	if err != nil {
		log.Fatalf("Fatal error: %s", err.Error())
	}
}
