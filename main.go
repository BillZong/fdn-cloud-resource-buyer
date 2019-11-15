package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	cli "gopkg.in/urfave/cli.v1"
)

const (
	configFileLong   = "config"
	nodeCountLong    = "node-count"
	showTemplateLong = "show-template"
	// 必选项
	regionIDLong        = "region-id"
	accessKeyIDLong     = "access-key-id"
	accessKeySecretLong = "access-key-secret"
	templateIDLong      = "template-id"
	// 可选项
	periodLong         = "period"
	periodUnitLong     = "period-unit"
	hostNamePrefixLong = "host-name-prefix"
	sshPortLong        = "ssh-port"
	sshKeyPairNameLong = "ssh-key-pair-name"
	sshKeyFileLong     = "ssh-key-file"
	passwordLong       = "password"
	// 调试选项
	debugKeyLong = "debug"
)

const (
	templateToShow = `cluster-type: "fixed" # fixed or dynamic. fixed one should provide existed node (not join to K8S and OW yet) list. dynamic one would use buy node process.
fixed:
  ssh-port: 12345
  user-name: "root"
  ssh-key-file: "./key-20191106" # use private key
  password: "123456Abc" # use password
  nodes:
    - info:
      inner-ip: "172.17.0.2"
      host-name: "a"
    - info:
      inner-ip: "172.17.0.3"
      host-name: "b"
    - info:
      inner-ip: "172.17.0.4"
      host-name: "c"
dynamic:
  cloud-provider: "aliyun"
  aliyun:
    # Required Parameters.
    # region id devided by aliyun
    region-id: "cn-shenzhen"
    # user acccess key id, might be RAM user
    access-key-id: "123456abcdef"
    # user access key secret
    access-key-secret: "asdfasdfasdf"
    # buy node template ID 
    template-id: "lt-lkjhasdfg"
    # Optional Parameters.
    # buy node period, no need when the ecs buying template use post-paid mode
    # period: 1
    # # buy node period unit, Month/Week，default "Month"
    # period-unit: "Week"
    # host name creation prefix, default "worker"
    host-name-prefix: "worker"
    # ssh port, default 20
    ssh-port: 12345
    # ssh key pair name created in ecs console. No need when use password login. Higher priority than password
    ssh-key-pair-name: "test-key-20191106"
    # ssh private key path, must needed when use ssh-key-pair-name
    ssh-key-file: "./key-20191106"
    # password, no need when use ssh private key login
    password: "123456Abc"
    # Debug Parameters
    # Debug mode. default false
    debug: false
# Command Line Parameters. Could be used in yaml, too
# # node count that want to be join. default 1
# node-count:
#   1`
)

func main() {
	app := cli.NewApp()

	app.Name = "node-joiner"
	app.Version = "0.1.0"
	app.Description = "Tools for invoker node (buy and) join in OpenWhisk cluster. First develeopped and used in FDN. Currently support fixed-number-nodes cluster, and aliyun ecs."
	app.Authors = []cli.Author{
		{Name: "Bill Zong", Email: "billzong@163.com"},
	}

	app.Commands = []cli.Command{
		// {
		// 	Name:  "join",
		// 	Usage: "options for join node to OpenWhisk cluster",
		// 	Flags: []cli.Flag{
		// 		cli.StringFlag{
		// 			Name:  "config,c",
		// 			Usage: "config file path, must have.",
		// 			Value: "./node-joiner-configs.yaml",
		// 		},
		// 		cli.IntFlag{
		// 			Name:  nodeCountLong,
		// 			Usage: "node count that want to be join",
		// 			Value: 1,
		// 		},
		// 	},
		// 	Action: startClient,
		// },
		{
			Name:  "template",
			Usage: "options for config yaml template",
			Subcommands: []cli.Command{
				{
					Name:   "show",
					Usage:  "show the template",
					Action: showTemplate,
				},
				{
					Name:  "create",
					Usage: "create (or cover) the tempalte to the path",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "path,p",
							Usage: "path for the config file, must have.",
							Value: "./node-joiner-configs.yaml",
						},
					},
					Action: createTemplate,
				},
			},
		},
	}
	// app.Action = startClient

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
	SSHKeyPairName *string `yaml:"ssh-key-pair-name,omitempty"`
	SSHKeyFile     *string `yaml:"ssh-key-file,omitempty"`
	Password       *string `yaml:"password,omitempty"`
	Debug          *bool   `yaml:"debug,omitempty"`
}

func startClient(ctx *cli.Context) error {
	var regionID, accessKey, accessSecret, templateID string
	var nodeCount, period, sshPort int
	var periodUnit, hostNamePrefix, sshKeyPairName, sshKeyFile, password string
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
		if cfg.SSHKeyPairName != nil {
			sshKeyPairName = *cfg.SSHKeyPairName
		} else {
			sshKeyPairName = ctx.String(sshKeyPairNameLong)
		}
		if cfg.SSHKeyFile != nil {
			sshKeyFile = *cfg.SSHKeyFile
		} else {
			sshKeyFile = ctx.String(sshKeyFileLong)
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
		sshKeyPairName = ctx.String(sshKeyPairNameLong)
		sshKeyFile = ctx.String(sshKeyFileLong)
		password = ctx.String(passwordLong)
		debugMode = ctx.Bool(debugKeyLong)
	}

	if len(sshKeyPairName) > 0 {
		if len(sshKeyFile) == 0 {
			return fmt.Errorf("need to set --ssh-key-file")
		}
	}

	client, err := ecs.NewClientWithAccessKey(regionID, accessKey, accessSecret)
	if err != nil {
		return err
	}

	// 创建实例
	instanceIds, err := runInstances(client, templateID, nodeCount, period, periodUnit, hostNamePrefix, sshKeyPairName, password, debugMode)
	if err != nil {
		return err
	}

	// 获取实例信息
	infos, err := checkInstancesInfo(client, instanceIds)
	if err != nil {
		return nil
	}

	// 连接实例并将它们加入OpenWhisk集群，阿里云的默认登陆账户为root
	if err := joinInstancesToOWCluster(infos, sshPort, "root", password, sshKeyFile); err != nil {
		return err
	}

	return nil
}

func joinInstancesToOWCluster(infos []nodeInfo, nodeSSHPort int, user, password, sshKeyFile string) error {
	var ips, names string
	for idx, info := range infos {
		ips += info.InnerIP
		names += info.HostName
		if idx < len(infos)-1 {
			ips += ","
			names += ","
		}
	}
	if len(sshKeyFile) > 0 {
		// 使用私钥文件ssh登陆
		_, err := exec.Command("./join-k8s.sh", "-h", ips, "-P", strconv.Itoa(nodeSSHPort), "-n", names, "-u", user, "-s", sshKeyFile).Output()
		return err
	}

	// 使用密码ssh登陆
	_, err := exec.Command("./join-k8s.sh", "-h", ips, "-P", strconv.Itoa(nodeSSHPort), "-n", names, "-u", user, "-p", password).Output()
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

func runInstances(client *ecs.Client, templateID string, nodeCount, period int, periodUnit string, hostNamePrefix, sshKeyPairName, password string, debugMode bool) ([]string, error) {
	// 创建请求并设置参数
	request := ecs.CreateRunInstancesRequest()
	request.LaunchTemplateId = templateID
	request.Amount = requests.NewInteger(nodeCount) // 购买台数
	request.Period = requests.NewInteger(period)    // 购买周期
	request.PeriodUnit = periodUnit                 // 周期单位，默认为月
	if len(sshKeyPairName) > 0 {
		request.KeyPairName = sshKeyPairName // 优先使用私钥文件登陆
	} else if len(password) > 0 {
		request.Password = password
	}
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

func createTemplate(ctx *cli.Context) error {
	path := ctx.String("path")
	if len(path) == 0 {
		path = "./node-joiner-configs.yaml"
	}
	return ioutil.WriteFile(path, []byte(templateToShow), 0644)
}

func showTemplate(ctx *cli.Context) error {
	fmt.Println("You could use this config yaml template:\n", templateToShow)
	return nil
}
