package main

import (
	"encoding/json"
	"gopkg.in/ini.v1"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"
)

var userStore = NewStore()

var cluster = NewCluster()

// 信息接收接口
func receive(resp http.ResponseWriter, req *http.Request) {
	userId := req.FormValue("userId")
	user := NewUser(userId, req.RemoteAddr)
	timer := time.NewTimer(time.Second * 20)
	us := userStore.AddUser(user)
	v, has := us.Wait(timer)
	us.UpdateLastLeaveTime(user)
	if has {
		msg := v.(string)
		resp.Write([]byte(msg))
	} else {
		resp.Write([]byte(""))
	}

}

// 信息发送接口
func send(resp http.ResponseWriter, req *http.Request) {
	msg := req.FormValue("msg")
	userId := req.FormValue("userId")
	us := userStore.GetUser(userId)
	if us != nil {
		us.Send(msg)
		resp.Write([]byte("ok"))
	} else {
		isOK := cluster.SendMsg(userId, msg)
		if isOK {
			resp.Write([]byte("ok"))
		} else {
			resp.Write([]byte("no user"))
		}
	}

}

// 集群握手接口
func initial(w http.ResponseWriter, re *http.Request) {
	machine, err := getRemoteMachine(re)
	if err != nil {
		return
	} else {
		cluster.addNewAddress(machine.MachineId, machine.RemoteAddress)
		localMachine := cluster.getLocalMachine()
		data, err := json.Marshal(localMachine)
		if err != nil {
			return
		} else {
			w.Write(data)
		}
	}

}

func getRemoteMachine(re *http.Request) (*Machine, error) {
	all, err := io.ReadAll(re.Body)
	if err != nil {
		return nil, err
	} else {
		var machine Machine
		err := json.Unmarshal(all, &machine)
		if err != nil {
			return nil, err
		}
		_, port, err := net.SplitHostPort(machine.RemoteAddress)
		if err != nil {
			return nil, err
		}
		host, _, err := net.SplitHostPort(re.RemoteAddr)
		if err != nil {
			return nil, err
		}
		remoteAddress := host + ":" + port
		machine.RemoteAddress = remoteAddress
		return &machine, nil
	}
}

// 集群查询接口
func queryMachine(w http.ResponseWriter, re *http.Request) {
	machine, err := getRemoteMachine(re)
	if err != nil {
		return
	} else {
		cluster.addNewAddress(machine.MachineId, machine.RemoteAddress)
		machines := cluster.machineList.getMachines()
		marshal, err := json.Marshal(machines)
		if err != nil {
			return
		} else {
			w.Write(marshal)
		}
	}
}

// 查询接口，验证是否已经有所有集群机器
func queryMachine2(w http.ResponseWriter, re *http.Request) {
	machines := cluster.machineList.getMachines()
	marshal, err := json.Marshal(machines)
	if err != nil {
		return
	} else {
		w.Write(marshal)
	}
}
func main() {
	//读取配置文件
	cfg, err := ini.Load("config.ini")
	if err != nil {
		log.Panic(err)
	}
	portKey := cfg.Section("core").Key("port")
	port, err := portKey.Int()
	if err != nil {
		log.Panic(err)
	}
	remoteAddressKey := cfg.Section("cluster").Key("remote-address") //首台要连接的机器
	remoteAddress := remoteAddressKey.Value()
	machineIdKey := cfg.Section("cluster").Key("machineId") //当前机器的唯一ID，每一台机器必须不一样
	machineId := machineIdKey.Value()

	go userStore.loopCheck() //用户离线检查

	http.HandleFunc("/receive", receive)
	http.HandleFunc("/send", send)

	cluster.init(machineId, remoteAddress, port)

	go cluster.run() //启动集群

	http.HandleFunc("/_cluster/initial", initial)           //集群握手接口
	http.HandleFunc("/_cluster/queryMachine", queryMachine) //集群机器列表查询接口
	http.HandleFunc("/queryMachine", queryMachine2)         //查询机器列表接口
	err = http.ListenAndServe(":"+strconv.Itoa(port), nil)
	if err != nil {
		log.Panic(err)
	}
}
