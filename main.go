package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	cli "gopkg.in/urfave/cli.v1"
)

const (
	// 必选项
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
	// 可选项
	nodeCountLong      = "node-count"
	periodLong         = "period"
	periodUnitLong     = "period-unit"
	hostNamePrefixLong = "host-name-prefix"
	sshPortLong        = "ssh-port"
	accountLong        = "account"
	passwordLong       = "password"
	// 调试选项
	debugKeyLong = "debug"
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
			Name:  nodeCountLong,
			Usage: "采购节点数量，默认为1",
			Value: 1,
		},
		cli.IntFlag{
			Name:  periodLong,
			Usage: "采购周期数量，单位参考period-unit。如果采用的采购模板为按量付费，则该选项无效",
			Value: 1,
		},
		cli.StringFlag{
			Name:  periodUnitLong,
			Usage: "采购周期单位，Month/Week，默认为Month。如果采用的采购模板为按量付费，则该选项无效",
			Value: "Month",
		},
		cli.StringFlag{
			Name:  hostNamePrefixLong,
			Usage: "节点的名称前缀，默认为fdn-worker",
			Value: "fdn-worker",
		},
		cli.Int64Flag{
			Name:  sshPortLong,
			Usage: "节点的SSH端口号，默认20",
			Value: 20,
		},
		cli.StringFlag{
			Name:  accountLong,
			Usage: "节点的账户，默认root",
			Value: "root",
		},
		cli.StringFlag{
			Name:  passwordLong,
			Usage: "节点的密码，默认123456Abc",
			Value: "123456Abc",
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
	RegionId       string  `yaml:"region-id"`
	AccessId       string  `yaml:"access-key-id"`
	AccessSecret   string  `yaml:"access-key-secret"`
	TemplateID     string  `yaml:"template-id"`
	NodeCount      *int    `yaml:"node-count,omitempty"`
	Period         *int    `yaml:"period,omitempty"`
	PeriodUnit     *string `yaml:"period-unit,omitempty"`
	HostNamePrefix *string `yaml:"host-name-prefix,omitempty"`
	SSHPort        *int    `yaml:"ssh-port,omitempty"`
	Account        *string `yaml:"account,omitempty"`
	Password       *string `yaml:"password,omitempty"`
	Debug          *bool   `yaml:"debug,omitempty"`
}

func startClient(ctx *cli.Context) error {
	var regionID, accessKey, accessSecret, templateID string
	var nodeCount, period, sshPort int
	var periodUnit, hostNamePrefix, account, password string
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
		if cfg.HostNamePrefix != nil {
			hostNamePrefix = *cfg.HostNamePrefix
		} else {
			hostNamePrefix = ctx.String(hostNamePrefixLong)
		}
		if cfg.SSHPort != nil {
			sshPort = *cfg.SSHPort
		} else {
			sshPort = ctx.Int(sshPortLong)
		}
		if cfg.Account != nil {
			account = *cfg.Account
		} else {
			account = ctx.String(accountLong)
		}
		if cfg.Password != nil {
			password = *cfg.Password
		} else {
			password = ctx.String(passwordLong)
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
		hostNamePrefix = ctx.String(hostNamePrefixLong)
		sshPort = ctx.Int(sshPortLong)
		account = ctx.String(accountLong)
		password = ctx.String(passwordLong)
		debugMode = ctx.Bool(debugKeyLong)
	}

	client, err := ecs.NewClientWithAccessKey(regionID, accessKey, accessSecret)
	if err != nil {
		return err
	}

	// 创建实例
	instanceIds, err := runInstances(client, templateID, nodeCount, period, periodUnit, hostNamePrefix, account, password, debugMode)
	if err != nil {
		return err
	}

	// 获取实例信息
	infos, err := checkInstancesInfo(client, instanceIds)
	if err != nil {
		return nil
	}

	// 连接实例并将它们加入OpenWhisk集群
	if err := joinInstancesToOWCluster(infos, sshPort, account, password); err != nil {
		return err
	}

	return nil
}

func joinInstancesToOWCluster(infos []nodeInfo, nodeSSHPort int, user, password string) error {
	var ips, names string
	for idx, info := range infos {
		ips += info.InnerIP
		names += info.HostName
		if idx < len(infos)-1 {
			ips += ","
			names += ","
		}
	}

	output, err := exec.Command("./join-k8s.sh", "-h", ips, "-P", strconv.Itoa(nodeSSHPort), "-n", names, "-u", user, "-p", password).Output()
	if err == nil {
		fmt.Printf("output: %v\n", string(output))
	}

	return err
}

type nodeInfo struct {
	InstanceID string
	InnerIP    string
	HostName   string
	createTime string //iso8601, UTC
}

func checkInstancesInfo(client *ecs.Client, instanceIds []string) ([]nodeInfo, error) {
	request := ecs.CreateDescribeInstancesRequest()
	ids, err := json.Marshal(instanceIds)
	if err != nil {
		return nil, err
	}
	request.InstanceIds = string(ids)
	response, err := client.DescribeInstances(request)
	if err != nil {
		return nil, err
	}

	instances := response.Instances.Instance
	infos := make([]nodeInfo, 0, len(instances))
	for _, instance := range instances {
		fmt.Printf("我们要保存节点ID、内网IP、主机名、创建时间，分别为: %v, %v, %v, %v\n", instance.InstanceId, instance.InnerIpAddress.IpAddress, instance.HostName, instance.CreationTime)
		infos = append(infos, nodeInfo{instance.InstanceId, instance.InnerIpAddress.IpAddress[0], instance.HostName, instance.CreationTime})
	}

	return infos, nil
}

func runInstances(client *ecs.Client, templateID string, nodeCount, period int, periodUnit string, hostNamePrefix, account, password string, debugMode bool) ([]string, error) {
	// 创建请求并设置参数
	request := ecs.CreateRunInstancesRequest()
	request.LaunchTemplateId = templateID
	request.Amount = requests.NewInteger(nodeCount) // 购买台数
	request.Period = requests.NewInteger(period)    // 购买周期
	request.PeriodUnit = periodUnit                 // 周期单位，默认为月
	request.Password = password
	t := time.Now().Format("2006-01-02-15-04")
	request.InstanceName = fmt.Sprintf("%v-dynamic-%v-[%v,3]", hostNamePrefix, t, nodeCount)
	request.HostName = fmt.Sprintf("%v-%v-[%v,3]", hostNamePrefix, t, nodeCount)
	if debugMode {
		request.DryRun = requests.NewBoolean(true) // 调试模式
	}
	request.ClientToken = t // 同1分钟只允许扩容一次

	response, err := client.RunInstances(request) // 发布请求
	if err != nil {
		// 异常处理
		return nil, err
	}
	return response.InstanceIdSets.InstanceIdSet, nil
}
