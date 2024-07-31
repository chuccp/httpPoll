package main

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Machine struct {
	RemoteAddress string
	MachineId     string
}
type Machines struct {
	machines []*Machine
	lock     *sync.RWMutex
}

// 添加首台机器
func (m *Machines) addFirstAddress(RemoteAddress string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, machine := range m.machines {
		if machine.RemoteAddress == RemoteAddress {
			return
		}
	}
	m.machines = append(m.machines, &Machine{RemoteAddress: RemoteAddress})
}

// 获取集群列表
func (m *Machines) getMachines() []*Machine {
	m.lock.RLock()
	defer m.lock.RUnlock()
	var machines = make([]*Machine, len(m.machines))
	copy(machines[:], m.machines[:])
	return machines
}

// 判断是否存在机器
func (m *Machines) hasMachines(machineId string, RemoteAddress string) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()
	for _, machine := range m.machines {
		if machine.RemoteAddress == RemoteAddress || machine.MachineId == machineId {
			return true
		}
	}
	return false
}

// 移除机器
func (m *Machines) removeMachine(RemoteAddress string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	var machines = make([]*Machine, 0)
	for _, machine := range m.machines {
		if machine.RemoteAddress != RemoteAddress {
			machines = append(machines, machine)
		}
	}
	m.machines = machines
}

// 添加机器
func (m *Machines) addMachines(RemoteAddress string, MachineId string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, machine := range m.machines {
		if machine.RemoteAddress == RemoteAddress || machine.MachineId == MachineId {
			return
		}
	}
	m.machines = append(m.machines, &Machine{RemoteAddress: RemoteAddress, MachineId: MachineId})
}

func NewMachines() *Machines {
	return &Machines{machines: make([]*Machine, 0), lock: new(sync.RWMutex)}
}

type Cluster struct {
	MachineId       string
	RemoteAddress   string
	LocalPort       int
	tempMachineList *Machines
	machineList     *Machines
	request         *Request
}

// 获取本机的机器信息
func (c *Cluster) getLocalMachine() *Machine {
	localAddress := "0.0.0.0:" + strconv.Itoa(c.LocalPort)
	return &Machine{RemoteAddress: localAddress, MachineId: c.MachineId}
}

// 添加首台机器
func (c *Cluster) addFirstAddress(RemoteAddress string) {
	c.tempMachineList.addFirstAddress(RemoteAddress)
}

// 添加新的机器
func (c *Cluster) addNewAddress(MachineId string, RemoteAddress string) {
	if !c.machineList.hasMachines(MachineId, RemoteAddress) {
		c.tempMachineList.addMachines(RemoteAddress, MachineId)
	}
}

// 初始化集群信息
func (c *Cluster) init(MachineId string, RemoteAddress string, LocalPort int) {
	c.LocalPort = LocalPort
	c.MachineId = MachineId
	c.RemoteAddress = RemoteAddress
	c.addFirstAddress(RemoteAddress)
}

// 发起握手
func (c *Cluster) initial(RemoteAddress string) (error, *Machine) {
	machine := c.getLocalMachine()
	data, err := json.Marshal(machine)
	if err != nil {
		return err, nil
	}
	call, err := c.request.Call("http://"+RemoteAddress+"/_cluster/initial", data)
	if err != nil {
		return err, nil
	}
	var _machine_ Machine
	err = json.Unmarshal(call, &_machine_)
	if err != nil {
		return err, nil
	}
	return err, &_machine_
}

// 发起查询
func (c *Cluster) queryMachine(RemoteAddress string) (error, []*Machine) {
	localAddress := "0.0.0.0:" + strconv.Itoa(c.LocalPort)
	machine := &Machine{MachineId: c.MachineId, RemoteAddress: localAddress}
	data, err := json.Marshal(machine)
	if err != nil {
		return err, nil
	}
	call, err := c.request.Call("http://"+RemoteAddress+"/_cluster/queryMachine", data)
	if err != nil {
		return err, nil
	}
	var _machines_ []*Machine
	err = json.Unmarshal(call, &_machines_)
	if err != nil {
		return err, nil
	}
	return err, _machines_
}

// SendMsg2 向集群机器发送消息
func (c *Cluster) SendMsg2(userId, msg string) bool {
	for _, machine := range c.machineList.machines {
		get, err := c.request.Get("http://" + machine.RemoteAddress + "/send?userId=" + userId + "&msg=" + msg)
		if err != nil {
			continue
		}
		if strings.Contains(string(get), "ok") {
			return true
		}
	}
	return false
}
func (c *Cluster) SendMsg(userId, msg string) bool {
	waitGroup := new(sync.WaitGroup)
	isSuccess := false
	for _, machine := range c.machineList.machines {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			get, err := c.request.Get("http://" + machine.RemoteAddress + "/send?userId=" + userId + "&msg=" + msg)
			if err == nil {
				if strings.Contains(string(get), "ok") {
					isSuccess = true
				}
			}

		}()
	}
	waitGroup.Wait()
	return isSuccess
}

// 将机器节点从临时列表。移到正式列表
func (c *Cluster) switchTempToList(machine *Machine) {
	c.tempMachineList.removeMachine(machine.RemoteAddress)
	c.machineList.addMachines(machine.RemoteAddress, machine.MachineId)
}

// 将自身的信息与目标机器进行数据校验，如果
func (c *Cluster) initials() {
	machines := c.tempMachineList.getMachines()
	for _, machine := range machines {
		log.Println("initials", machine.RemoteAddress, machine.MachineId)
		err, m := c.initial(machine.RemoteAddress)
		if err != nil {
			continue
		}
		if m.MachineId == c.MachineId {
			c.tempMachineList.removeMachine(machine.RemoteAddress)
		} else {
			if machine.MachineId != "" && machine.MachineId != m.MachineId {
				machine.MachineId = m.MachineId
			}
			c.switchTempToList(machine)
		}
	}
}
func (c *Cluster) queryMachines() {
	machines := c.machineList.getMachines()
	for _, machine := range machines {
		err, ms := c.queryMachine(machine.RemoteAddress)
		if err != nil {
			continue
		}
		for _, m := range ms {
			if m.MachineId == c.MachineId {
				continue
			} else {
				c.addNewAddress(m.MachineId, m.RemoteAddress)
			}
		}
	}
}

// 循环执行握手和查询任务
func (c *Cluster) loop() {
	for {
		time.Sleep(time.Second)
		///检查临时节点
		c.initials()
		time.Sleep(time.Second)
		c.queryMachines()
		time.Sleep(30 * time.Second)
	}

}

func (c *Cluster) run() {
	go c.loop()

}

func NewCluster() *Cluster {

	return &Cluster{request: NewRequest(), machineList: NewMachines(), tempMachineList: NewMachines()}
}
