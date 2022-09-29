package main

import (
	"Open_IM/internal/rpc/rtc"
	"Open_IM/pkg/common/config"
	promePkg "Open_IM/pkg/common/prometheus"
	"flag"
	"fmt"
)

func main() {
	defaultPorts := config.Config.RpcPort.OpenImRealTimeCommPort
	rpcPort := flag.Int("port", defaultPorts[0], "rpc listening port")
	prometheusPort := flag.Int("prometheus_port", config.Config.Prometheus.RealTimeCommPrometheusPort[0], "real_time_commPrometheusPort default listen port")
	flag.Parse()
	fmt.Println("start real_time_comm rpc server, port: ", *rpcPort)
	rpcServer := rtc.NewRpcLiveKitServer(*rpcPort)
	go func() {
		err := promePkg.StartPromeSrv(*prometheusPort)
		if err != nil {
			panic(err)
		}
	}()
	rpcServer.Run()
}
