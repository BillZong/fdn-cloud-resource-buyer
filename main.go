package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/errors"
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
	}
	app.Action = startClient

	err := app.Run(os.Args)
	if err != nil {
		if serverErr, isServerErr := err.(*errors.ServerError); isServerErr {
			fmt.Printf("得到阿里云的服务器错误提示: %s\n\n", serverErr)
		} else {
			fmt.Println(err)
		}
	}
}

func startClient(ctx *cli.Context) error {
	regionID := ctx.String(regionIDLong)
	accessKey := ctx.String(accessKeyIDLong)
	accessSecret := ctx.String(accessKeySecretLong)
	client, err := ecs.NewClientWithAccessKey(regionID, accessKey, accessSecret)
	if err != nil {
		return err
	}

	// 创建请求并设置参数
	request := ecs.CreateRunInstancesRequest()
	request.LaunchTemplateId = "lt-wz94jxtokyiy1r51gk4a"
	// request.ImageId = "ubuntu_18_04_64_20G_alibase_20190624.vhd" // Ubuntu 18, 64位
	// request.InstanceType = "ecs.sn1ne.large"                     // 计算型，2核4G，X86架构
	// // 必须要有安全组
	// request.SecurityGroupId = "sg-wz95w0u4m3yxgh68nwy3"
	// request.ZoneId = "cn-shenzhen-a"
	// request.ClientToken = utils.GetUUID()
	// request.Amount = requests.NewInteger(5) // 购买台数
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

	request.Period = requests.NewInteger(1)
	request.DryRun = requests.NewBoolean(true) // 调试用
	response, err := client.RunInstances(request)
	if err != nil {
		// 异常处理
		return err
	}
	fmt.Printf("success(%d)! instanceId = %s\n", response.GetHttpStatus(), strings.Join(response.InstanceIdSets.InstanceIdSet, ","))

	return nil
}
