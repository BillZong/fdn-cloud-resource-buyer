package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	cli "gopkg.in/urfave/cli.v1"
)

const (
	regionIDLong         = "region-id"
	regionIDShort        = "r"
	accessKeyIDLong      = "access-key-id"
	accessKeyIDShort     = "k"
	accessKeySecretLong  = "access-key-secret"
	accessKeySecretShort = "s"
	configFileLong       = "config"
	configFileShort      = "c"
	templateIDLong       = "template-id"
	templateIDShort      = "t"
	nodeCountLong        = "node-count"
	nodeCountShort       = "n"
	periodLong           = "period"
	periodShort          = "p"
	periodUnitLong       = "period-unit"
	periodUnitShort      = "u"
	debugKeyLong         = "debug"
)

func main() {
	app := cli.NewApp()

	app.Name = "AliyunECSBuyer"
	app.Version = "0.0.1"
	app.Description = "fdn aliyun 资源购买工具"
	app.Authors = []cli.Author{
		{Name: "FDN developper"},
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config,c",
			Usage: "配置文件路径，使用后不参考其它参数配置",
		},
		cli.StringFlag{
			Name:  "region-id,r",
			Usage: "区域编号，要求一定要有",
		},
		cli.StringFlag{
			Name:  "access-key-id,k",
			Usage: "授权KeyID",
		},
		cli.StringFlag{
			Name:  "access-key-secret,s",
			Usage: "授权Key秘钥",
		},
		cli.StringFlag{
			Name:  "template-id,t",
			Usage: "采购模板ID",
		},
		cli.IntFlag{
			Name:  "node-count,n",
			Usage: "采购节点数量",
			Value: 1,
		},
		cli.IntFlag{
			Name:  "period,p",
			Usage: "采购周期数量，单位参考period-unit",
			Value: 1,
		},
		cli.StringFlag{
			Name:  "period-unit,u",
			Usage: "采购周期单位，Month/Week，默认为Month",
			Value: "Month",
		},
		cli.BoolFlag{
			Name:  debugKeyLong,
			Usage: "调试用，不直接运行",
		},
	}
	app.Action = startClient

	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}

type EcsConfig struct {
	RegionId     string  `yaml:"region-id"`
	AccessId     string  `yaml:"access-key-id"`
	AccessSecret string  `yaml:"access-key-secret"`
	TemplateID   string  `yaml:"template-id"`
	NodeCount    *int    `yaml:"node-count,omitempty"`
	Period       *int    `yaml:"period,omitempty"`
	PeriodUnit   *string `yaml:"period-unit,omitempty"`
	Debug        *bool   `yaml:"debug,omitempty"`
}

func startClient(ctx *cli.Context) error {
	var regionID, accessKey, accessSecret string
	var templateID string
	var nodeCount, period int
	var periodUnit string
	debugMode := false
	if path := ctx.String(configFileLong); len(path) > 0 {
		var cfg EcsConfig
		if err := ReadYamlFile(path, &cfg); err != nil {
			return err
		}
		regionID = cfg.RegionId
		accessKey = cfg.AccessId
		accessSecret = cfg.AccessSecret
		templateID = cfg.TemplateID

		// optional args for default value
		if cfg.NodeCount != nil {
			nodeCount = *cfg.NodeCount
		} else {
			nodeCount = ctx.Int(nodeCountLong)
		}
		if cfg.Period != nil {
			period = *cfg.Period
		} else {
			period = ctx.Int(periodLong)
		}
		if cfg.PeriodUnit != nil {
			periodUnit = *cfg.PeriodUnit
		} else {
			periodUnit = ctx.String(periodUnitLong)
		}

		if cfg.Debug != nil {
			debugMode = *cfg.Debug
		}
	} else {
		regionID = ctx.String(regionIDLong)
		accessKey = ctx.String(accessKeyIDLong)
		accessSecret = ctx.String(accessKeySecretLong)
		templateID = ctx.String(templateIDLong)
		nodeCount = ctx.Int(nodeCountLong)
		period = ctx.Int(periodLong)
		periodUnit = ctx.String(periodUnitLong)
		debugMode = ctx.Bool(debugKeyLong)
	}

	client, err := ecs.NewClientWithAccessKey(regionID, accessKey, accessSecret)
	if err != nil {
		return err
	}

	// 创建请求并设置参数
	request := ecs.CreateRunInstancesRequest()
	request.LaunchTemplateId = templateID
	request.Amount = requests.NewInteger(nodeCount) // 购买台数
	request.Period = requests.NewInteger(period)    // 购买周期
	request.PeriodUnit = periodUnit                 // 周期单位，默认为月
	if debugMode {
		request.DryRun = requests.NewBoolean(true) // 调试模式
	}

	// request.ImageId = "ubuntu_18_04_64_20G_alibase_20190624.vhd" // Ubuntu 18, 64位
	// request.InstanceType = "ecs.sn1ne.large"                     // 计算型，2核4G，X86架构
	// // 必须要有安全组
	// request.SecurityGroupId = "sg-wz95w0u4m3yxgh68nwy3"
	// request.ZoneId = "cn-shenzhen-a"
	// request.ClientToken = utils.GetUUID()
	// // // 指定标签
	// // request.Tag = &[]ecs.RunInstancesTag{
	// // 	ecs.RunInstancesTag{
	// // 		Key:   "tag-for-test",
	// // 		Value: "123",
	// // 	},
	// // }
	// // 指定后缀
	// request.InstanceName = "MyTestInstance[1,2]" // 实例名称，MyTestInstance01, MyTestInstance02, ...
	// request.HostName = "MyTestHost[1,2]"         // MyTestHost01, MyTestHost02, ...
	// request.UniqueSuffix = requests.NewBoolean(true)
	// request.Password = "Ab123456"

	response, err := client.RunInstances(request) // 发布请求
	if err != nil {
		// 异常处理
		return err
	}
	fmt.Printf("success(%d)! instanceId = %s, instance amount %v\n", response.GetHttpStatus(), strings.Join(response.InstanceIdSets.InstanceIdSet, ","), len(response.InstanceIdSets.InstanceIdSet))

	return nil
}
