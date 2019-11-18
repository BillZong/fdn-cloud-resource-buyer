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
	configFilePathLongFlag = "config"
	nodeCountLongFlag      = "node-count"
)

const (
	templateToShow = `cluster-type: "fixed" # fixed or dynamic. fixed one should provide existed node (not join to K8S and OW yet) list. dynamic one would use buy node process.
fixed:
  ssh-port: 12345
  user-name: "root"
  ssh-key-file: "./key-20191106" # use private key
  password: "123456Abc" # use password
  nodes:
    - inner-ip: "172.17.0.2"
      host-name: "a"
    - inner-ip: "172.17.0.3"
      host-name: "b"
    - inner-ip: "172.17.0.4"
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
    # # buy node period unit, Month/Week
    # period-unit: "Week"
    # host name creation prefix, default "worker"
    host-name-prefix: "worker"
    # ssh port, default 22
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
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config,c",
			Usage: "config file path, must have.",
			Value: "./node-joiner-configs.yaml",
		},
		cli.IntFlag{
			Name:  nodeCountLongFlag,
			Usage: "node count that want to be join",
			Value: 1,
		},
	}
	app.Action = joinOWCluster

	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}

type NodeInfo struct {
	InnerIP  string `yaml:"inner-ip"`
	HostName string `yaml:"host-name"`
}

type FixedNodeConfig struct {
	SSHPort    int         `yaml:"ssh-port,omitempty"`
	UserName   string      `yaml:"user-name,omitempty"`
	SSHKeyFile string      `yaml:"ssh-key-file,omitempty"`
	Password   string      `yaml:"password,omitempty"`
	Nodes      []*NodeInfo `yaml:"nodes"`
}

type AliyunEcsConfig struct {
	RegionID       string  `yaml:"region-id"`
	AccessID       string  `yaml:"access-key-id"`
	AccessSecret   string  `yaml:"access-key-secret"`
	TemplateID     string  `yaml:"template-id"`
	Period         *int    `yaml:"period,omitempty"`
	PeriodUnit     *string `yaml:"period-unit,omitempty"`
	HostNamePrefix *string `yaml:"host-name-prefix,omitempty"`
	SSHPort        *int    `yaml:"ssh-port,omitempty"`
	SSHKeyPairName *string `yaml:"ssh-key-pair-name,omitempty"`
	SSHKeyFile     *string `yaml:"ssh-key-file,omitempty"`
	Password       *string `yaml:"password,omitempty"`
	Debug          *bool   `yaml:"debug,omitempty"`
}

type DynamicNodeConfig struct {
	CloudProvider string           `yaml:"cloud-provider"`
	AliyunConfig  *AliyunEcsConfig `yaml:"aliyun,omitempty"`
}

type TopLevelConfigs struct {
	ClusterType   string             `yaml:"cluster-type"`
	FixedConfig   *FixedNodeConfig   `yaml:"fixed,omitempty"`
	DynamicConfig *DynamicNodeConfig `yaml:"dynamic,omitempty"`
	NodeCount     *int               `yaml:"node-count,omitempty"`
}

func joinOWCluster(ctx *cli.Context) error {
	var cfg = TopLevelConfigs{
		ClusterType: "fixed",
		FixedConfig: &FixedNodeConfig{
			SSHPort:  22,
			UserName: "root",
		},
	}
	configPath := ctx.String(configFilePathLongFlag)
	if len(configPath) == 0 {
		return fmt.Errorf("config file not existed")
	}
	if err := ReadYamlFile(configPath, &cfg); err != nil {
		return err
	}
	if cfg.NodeCount == nil {
		nodeCount := ctx.Int(nodeCountLongFlag)
		cfg.NodeCount = &nodeCount
	}

	if cfg.ClusterType == "fixed" {
		//TODO: fixed type configs
	} else if cfg.ClusterType == "dynamic" {
		if cfg.DynamicConfig.CloudProvider != "aliyun" {
			return fmt.Errorf("cloud provider (%v) not supported yet", cfg.DynamicConfig.CloudProvider)
		}
		if err := handleAliyunECSBuyConfigs(cfg.DynamicConfig.AliyunConfig, *cfg.NodeCount); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("cluster type (%v) not supported yet", cfg.ClusterType)
	}

	return nil
}

func handleAliyunECSBuyConfigs(cfg *AliyunEcsConfig, nodeCount int) error {
	if cfg.SSHKeyPairName != nil && len(*cfg.SSHKeyPairName) > 0 {
		if cfg.SSHKeyFile == nil || len(*cfg.SSHKeyFile) == 0 {
			return fmt.Errorf("need to set --ssh-key-file")
		}
	}

	client, err := ecs.NewClientWithAccessKey(cfg.RegionID, cfg.AccessID, cfg.AccessSecret)
	if err != nil {
		return err
	}

	// 创建实例
	instanceIds, err := runAliyunInstances(client, cfg, nodeCount)
	if err != nil {
		return err
	}

	// 获取实例信息
	infos, err := checkInstancesInfo(client, instanceIds)
	if err != nil {
		return nil
	}

	var port int
	if cfg.SSHPort != nil {
		port = *cfg.SSHPort
	} else {
		port = 22
	}

	// 连接实例并将它们加入OpenWhisk集群，阿里云的默认登陆账户为root
	if err := joinInstancesToOWCluster(infos, port, "root", cfg.Password, cfg.SSHKeyFile); err != nil {
		return err
	}

	return nil
}

func joinInstancesToOWCluster(infos []nodeInfo, nodeSSHPort int, user string, sshKeyFile, password *string) error {
	if sshKeyFile == nil && password == nil {
		return fmt.Errorf("no key, could not login to nodes")
	}

	var ips, names string
	for idx, info := range infos {
		ips += info.InnerIP
		names += info.HostName
		if idx < len(infos)-1 {
			ips += ","
			names += ","
		}
	}
	if len(*sshKeyFile) > 0 {
		// 使用私钥文件ssh登陆
		_, err := exec.Command("./join-k8s.sh", "-h", ips, "-P", strconv.Itoa(nodeSSHPort), "-n", names, "-u", user, "-s", *sshKeyFile).Output()
		return err
	}

	// 使用密码ssh登陆
	_, err := exec.Command("./join-k8s.sh", "-h", ips, "-P", strconv.Itoa(nodeSSHPort), "-n", names, "-u", user, "-p", *password).Output()
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

func runAliyunInstances(client *ecs.Client, cfg *AliyunEcsConfig, nodeCount int) ([]string, error) {
	if cfg == nil {
		return nil, fmt.Errorf("aliyun configs not set")
	}

	// create request and supply default value
	request := ecs.CreateRunInstancesRequest()
	request.LaunchTemplateId = cfg.TemplateID
	request.Amount = requests.NewInteger(nodeCount)
	if cfg.Period != nil {
		request.Period = requests.NewInteger(*cfg.Period) // node buying period
	}
	if cfg.PeriodUnit != nil {
		request.PeriodUnit = *cfg.PeriodUnit // node buying period unit
	}
	if cfg.SSHKeyPairName != nil && len(*cfg.SSHKeyPairName) > 0 {
		request.KeyPairName = *cfg.SSHKeyPairName // ssh key first
	} else if cfg.Password != nil && len(*cfg.Password) > 0 {
		request.Password = *cfg.Password // password second
	} else {
		return nil, fmt.Errorf("aliyun login key not set")
	}
	hostNamePrefix := "worker" // default host name prefix
	if cfg.HostNamePrefix != nil && len(*cfg.HostNamePrefix) > 0 {
		hostNamePrefix = *cfg.HostNamePrefix
	}
	t := time.Now().Format("2006-01-02-15-04") // time format to distinguish between node names
	request.InstanceName = fmt.Sprintf("%v-%v-[%v,3]", hostNamePrefix, t, nodeCount)
	request.HostName = fmt.Sprintf("%v-%v-[%v,3]", hostNamePrefix, t, nodeCount)
	request.ClientToken = t // only one creation in the same minute
	if cfg.Debug != nil && *cfg.Debug {
		request.DryRun = requests.NewBoolean(true) // debug mode
	}

	// send request
	response, err := client.RunInstances(request)
	if err != nil {
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
